package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// parl_agenda.go — fetch upcoming parliamentary events via the parlament.ch
// agenda page's SharePoint Search endpoint. This surfaces future scheduled
// sessions and other events that the OData Session entity has not yet
// published.

const (
	agendaBase        = "https://www.parlament.ch"
	agendaContextPath = "/_api/contextinfo"
	agendaQueryPath   = "/en/services/_vti_bin/client.svc/ProcessQuery"
	// SourceId of the "AgendaAllShowAll" search result source configured
	// on the agenda page. Extracted from the site's JavaScript.
	agendaSourceID = "{d4659255-875e-4444-b356-2ed2ce7e439c}"
	// ListId of the Pages library on the Services web.
	agendaListID = "263e44d0-b5e1-4c28-bc00-6c894a4c30da"
	// Stable query group id the server expects. Value is arbitrary per
	// site; this one is emitted by the agenda page template.
	agendaQueryID = "5c701ab9-b60a-4f1a-8a25-78a4d25c798fDefault"
)

// ParlAgendaEvent is one row from the agenda search result. Only the
// fields we actually use are mapped; the payload contains many more
// managed properties.
type ParlAgendaEvent struct {
	Title            string    `json:"Title"`
	Path             string    `json:"Path"`
	EventDate        time.Time `json:"EventDate"`
	EndEventDate     time.Time `json:"EndEventDate,omitempty"`
	TitleDe          string    `json:"TitleDe,omitempty"`
	TitleFr          string    `json:"TitleFr,omitempty"`
	TitleIt          string    `json:"TitleIt,omitempty"`
	TitleEn          string    `json:"TitleEn,omitempty"`
	LocationDe       string    `json:"LocationDe,omitempty"`
	LocationEn       string    `json:"LocationEn,omitempty"`
	CategoryDe       string    `json:"CategoryDe,omitempty"`
	CategoryFr       string    `json:"CategoryFr,omitempty"`
	CategoryIt       string    `json:"CategoryIt,omitempty"`
	CategoryEn       string    `json:"CategoryEn,omitempty"`
	CommissionAbbrDe string    `json:"CommissionAbbrDe,omitempty"`
	CommissionAbbrEn string    `json:"CommissionAbbrEn,omitempty"`
}

// LocalizedTitle returns the title for the given language code ("de", "fr",
// "it", "en"), falling back to Title then to any non-empty variant.
func (e ParlAgendaEvent) LocalizedTitle(lang string) string {
	switch strings.ToLower(lang) {
	case "de":
		if e.TitleDe != "" {
			return e.TitleDe
		}
	case "fr":
		if e.TitleFr != "" {
			return e.TitleFr
		}
	case "it":
		if e.TitleIt != "" {
			return e.TitleIt
		}
	case "en":
		if e.TitleEn != "" {
			return e.TitleEn
		}
	}
	for _, v := range []string{e.TitleEn, e.TitleDe, e.TitleFr, e.TitleIt, e.Title} {
		if v != "" {
			return v
		}
	}
	return e.Title
}

// LocalizedCategory returns the event category label for the given language.
func (e ParlAgendaEvent) LocalizedCategory(lang string) string {
	switch strings.ToLower(lang) {
	case "de":
		if e.CategoryDe != "" {
			return e.CategoryDe
		}
	case "fr":
		if e.CategoryFr != "" {
			return e.CategoryFr
		}
	case "it":
		if e.CategoryIt != "" {
			return e.CategoryIt
		}
	case "en":
		if e.CategoryEn != "" {
			return e.CategoryEn
		}
	}
	for _, v := range []string{e.CategoryEn, e.CategoryDe, e.CategoryFr, e.CategoryIt} {
		if v != "" {
			return v
		}
	}
	return ""
}

// FetchAgendaEvents queries the SharePoint Search endpoint behind the
// parlament.ch agenda page. queryTemplate is a KQL fragment applied server
// side — for example `PdAgendaCategoryEN:Session` restricts to sessions; an
// empty value returns every upcoming agenda item. rowLimit caps results;
// pass 0 for the default (100).
func (c *Client) FetchAgendaEvents(queryTemplate string, rowLimit int) ([]ParlAgendaEvent, error) {
	if rowLimit <= 0 {
		rowLimit = 100
	}
	digest, err := c.fetchFormDigest()
	if err != nil {
		return nil, fmt.Errorf("fetching SharePoint form digest: %w", err)
	}

	body := buildAgendaProcessQuery(queryTemplate, rowLimit)
	raw, err := c.postProcessQuery(body, digest)
	if err != nil {
		return nil, err
	}
	return parseAgendaResults(raw)
}

// fetchFormDigest retrieves an anonymous X-RequestDigest from SharePoint.
func (c *Client) fetchFormDigest() (string, error) {
	req, err := http.NewRequest("POST", agendaBase+agendaContextPath, strings.NewReader(""))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json;odata=nometadata")
	req.Header.Set("Content-Length", "0")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("contextinfo HTTP %d: %s", resp.StatusCode, string(b))
	}
	var ci struct {
		FormDigestValue string `json:"FormDigestValue"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ci); err != nil {
		return "", err
	}
	if ci.FormDigestValue == "" {
		return "", fmt.Errorf("empty form digest")
	}
	return ci.FormDigestValue, nil
}

// postProcessQuery sends the ProcessQuery XML payload and returns the
// response body. No caching here because the form digest is short-lived.
func (c *Client) postProcessQuery(body, digest string) ([]byte, error) {
	req, err := http.NewRequest("POST", agendaBase+agendaQueryPath, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("X-RequestDigest", digest)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", "https://www.parlament.ch/en/services/agenda")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ProcessQuery HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// buildAgendaProcessQuery builds the verbose SharePoint CSOM XML request
// that the agenda page issues. The structure is sensitive: removing fields
// like ListId or SafeQueryPropertiesTemplateUrl causes anonymous rejection.
func buildAgendaProcessQuery(queryTemplate string, rowLimit int) string {
	if queryTemplate == "" {
		queryTemplate = "*"
	}
	// Escape minimally for XML attribute/text context.
	qt := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(queryTemplate)
	return `<Request xmlns="http://schemas.microsoft.com/sharepoint/clientquery/2009" SchemaVersion="15.0.0.0" LibraryVersion="16.0.0.0" ApplicationName="Javascript Library">` +
		`<Actions>` +
		`<ObjectPath Id="1" ObjectPathId="0" />` +
		`<SetProperty Id="2" ObjectPathId="0" Name="TimeZoneId"><Parameter Type="Number">4</Parameter></SetProperty>` +
		`<SetProperty Id="3" ObjectPathId="0" Name="QueryTemplate"><Parameter Type="String">` + qt + `</Parameter></SetProperty>` +
		`<ObjectPath Id="5" ObjectPathId="4" />` +
		`<Method Name="Add" Id="6" ObjectPathId="4"><Parameters><Parameter Type="String">PdEventDate</Parameter><Parameter Type="Number">1</Parameter></Parameters></Method>` +
		`<SetProperty Id="7" ObjectPathId="0" Name="Culture"><Parameter Type="Number">1033</Parameter></SetProperty>` +
		fmt.Sprintf(`<SetProperty Id="8" ObjectPathId="0" Name="RowsPerPage"><Parameter Type="Number">%d</Parameter></SetProperty>`, rowLimit) +
		fmt.Sprintf(`<SetProperty Id="9" ObjectPathId="0" Name="RowLimit"><Parameter Type="Number">%d</Parameter></SetProperty>`, rowLimit) +
		fmt.Sprintf(`<SetProperty Id="10" ObjectPathId="0" Name="TotalRowsExactMinimum"><Parameter Type="Number">%d</Parameter></SetProperty>`, rowLimit+1) +
		`<SetProperty Id="11" ObjectPathId="0" Name="SourceId"><Parameter Type="Guid">` + agendaSourceID + `</Parameter></SetProperty>` +
		`<ObjectPath Id="13" ObjectPathId="12" />` +
		agendaProp("14", "SourceName", "String", "AgendaAllShowAll") +
		agendaProp("15", "SourceLevel", "String", "SPSite") +
		`<SetProperty Id="24" ObjectPathId="0" Name="TrimDuplicates"><Parameter Type="Boolean">false</Parameter></SetProperty>` +
		agendaProp("25", "ListId", "String", agendaListID) +
		agendaPropInt("26", "ListItemId", 3) +
		agendaProp("30", "CrossGeoQuery", "String", "false") +
		`<SetProperty Id="32" ObjectPathId="0" Name="ClientType"><Parameter Type="String">UI</Parameter></SetProperty>` +
		`<SetProperty Id="35" ObjectPathId="0" Name="SafeQueryPropertiesTemplateUrl"><Parameter Type="String">querygroup://webroot/Pages/Agenda.aspx?groupname=Default</Parameter></SetProperty>` +
		`<SetProperty Id="36" ObjectPathId="0" Name="IgnoreSafeQueryPropertiesTemplateUrl"><Parameter Type="Boolean">false</Parameter></SetProperty>` +
		`<ObjectPath Id="39" ObjectPathId="38" />` +
		`<ExceptionHandlingScope Id="40"><TryScope Id="42">` +
		`<Method Name="ExecuteQueries" Id="44" ObjectPathId="38">` +
		`<Parameters>` +
		`<Parameter Type="Array"><Object Type="String">` + agendaQueryID + `</Object></Parameter>` +
		`<Parameter Type="Array"><Object ObjectPathId="0" /></Parameter>` +
		`<Parameter Type="Boolean">true</Parameter>` +
		`</Parameters></Method>` +
		`</TryScope><CatchScope Id="46" /></ExceptionHandlingScope>` +
		`</Actions>` +
		`<ObjectPaths>` +
		`<Constructor Id="0" TypeId="{80173281-fffd-47b6-9a49-312e06ff8428}" />` +
		`<Property Id="4" ParentId="0" Name="SortList" />` +
		`<Property Id="12" ParentId="0" Name="Properties" />` +
		`<Constructor Id="38" TypeId="{8d2ac302-db2f-46fe-9015-872b35f15098}" />` +
		`</ObjectPaths></Request>`
}

// agendaProp renders a QueryPropertyValue string property setter.
func agendaProp(id, name, typ, val string) string {
	_ = typ
	return `<Method Name="SetQueryPropertyValue" Id="` + id + `" ObjectPathId="12"><Parameters>` +
		`<Parameter Type="String">` + name + `</Parameter>` +
		`<Parameter TypeId="{b25ba502-71d7-4ae4-a701-4ca2fb1223be}">` +
		`<Property Name="BoolVal" Type="Boolean">false</Property>` +
		`<Property Name="IntVal" Type="Number">0</Property>` +
		`<Property Name="QueryPropertyValueTypeIndex" Type="Number">1</Property>` +
		`<Property Name="StrArray" Type="Null" />` +
		`<Property Name="StrVal" Type="String">` + val + `</Property>` +
		`</Parameter></Parameters></Method>`
}

// agendaPropInt renders a QueryPropertyValue int property setter.
func agendaPropInt(id, name string, val int) string {
	return fmt.Sprintf(`<Method Name="SetQueryPropertyValue" Id="%s" ObjectPathId="12"><Parameters>`+
		`<Parameter Type="String">%s</Parameter>`+
		`<Parameter TypeId="{b25ba502-71d7-4ae4-a701-4ca2fb1223be}">`+
		`<Property Name="BoolVal" Type="Boolean">false</Property>`+
		`<Property Name="IntVal" Type="Number">%d</Property>`+
		`<Property Name="QueryPropertyValueTypeIndex" Type="Number">2</Property>`+
		`<Property Name="StrArray" Type="Null" />`+
		`<Property Name="StrVal" Type="Null" />`+
		`</Parameter></Parameters></Method>`, id, name, val)
}

// sharepointDate parses /Date(1234567890000)/ into a time.Time.
var sharepointDateRE = regexp.MustCompile(`^/Date\((-?\d+)\)/$`)

func parseSharepointDate(s string) time.Time {
	m := sharepointDateRE.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}
	}
	ms, err := parseInt64(m[1])
	if err != nil {
		return time.Time{}
	}
	return time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond))
}

func parseInt64(s string) (int64, error) {
	var n int64
	sign := int64(1)
	i := 0
	if i < len(s) && s[i] == '-' {
		sign = -1
		i++
	}
	if i == len(s) {
		return 0, fmt.Errorf("empty number")
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int64(s[i]-'0')
	}
	return n * sign, nil
}

// parseAgendaResults walks the CSOM response and extracts agenda events.
func parseAgendaResults(raw []byte) ([]ParlAgendaEvent, error) {
	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("parsing CSOM response: %w", err)
	}
	// First element contains error info. Any CSOM error is surfaced there.
	if len(parts) > 0 {
		var first struct {
			ErrorInfo *struct {
				ErrorMessage string `json:"ErrorMessage"`
			} `json:"ErrorInfo"`
		}
		if err := json.Unmarshal(parts[0], &first); err == nil && first.ErrorInfo != nil {
			return nil, fmt.Errorf("SharePoint error: %s", first.ErrorInfo.ErrorMessage)
		}
	}
	// Check for ExceptionHandlingScope result objects (HasException).
	for _, p := range parts {
		var ex struct {
			HasException bool `json:"HasException"`
			ErrorInfo    *struct {
				ErrorMessage string `json:"ErrorMessage"`
			} `json:"ErrorInfo"`
		}
		if err := json.Unmarshal(p, &ex); err == nil && ex.HasException && ex.ErrorInfo != nil {
			return nil, fmt.Errorf("SharePoint query error: %s", ex.ErrorInfo.ErrorMessage)
		}
	}
	// Find ResultRows by walking the structure.
	var rawRows []map[string]any
	var walk func(json.RawMessage)
	walk = func(m json.RawMessage) {
		var v any
		if err := json.Unmarshal(m, &v); err != nil {
			return
		}
		var visit func(x any)
		visit = func(x any) {
			switch t := x.(type) {
			case map[string]any:
				if rr, ok := t["ResultRows"].([]any); ok {
					for _, r := range rr {
						if rm, ok := r.(map[string]any); ok {
							rawRows = append(rawRows, rm)
						}
					}
				}
				for _, v := range t {
					visit(v)
				}
			case []any:
				for _, v := range t {
					visit(v)
				}
			}
		}
		visit(v)
	}
	for _, p := range parts {
		walk(p)
	}
	events := make([]ParlAgendaEvent, 0, len(rawRows))
	for _, r := range rawRows {
		ev := ParlAgendaEvent{
			Title:            agendaStr(r, "Title"),
			Path:             agendaStr(r, "Path"),
			TitleDe:          agendaStr(r, "PdTitleDe"),
			TitleFr:          agendaStr(r, "PdTitleFr"),
			TitleIt:          agendaStr(r, "PdTitleIt"),
			TitleEn:          agendaStr(r, "PdTitleEn"),
			LocationDe:       agendaStr(r, "PdLocationDe"),
			LocationEn:       agendaStr(r, "PdLocationEn"),
			CategoryDe:       agendaStr(r, "PdAgendaCategoryDE"),
			CategoryFr:       agendaStr(r, "PdAgendaCategoryFR"),
			CategoryIt:       agendaStr(r, "PdAgendaCategoryIT"),
			CategoryEn:       agendaStr(r, "PdAgendaCategoryEN"),
			CommissionAbbrDe: agendaStr(r, "PdCommissionAbbreviationDe"),
			CommissionAbbrEn: agendaStr(r, "PdCommissionAbbreviationEn"),
		}
		ev.EventDate = parseSharepointDate(agendaStr(r, "PdEventDate"))
		ev.EndEventDate = parseSharepointDate(agendaStr(r, "PdEndEventDate"))
		events = append(events, ev)
	}
	return events, nil
}

func agendaStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
