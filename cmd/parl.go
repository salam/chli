package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/matthiasak/chli/api"
	"github.com/matthiasak/chli/output"
	"github.com/spf13/cobra"
)

func newParlClient() (*api.Client, error) {
	client, err := api.NewClient()
	if err != nil {
		return nil, err
	}
	client.NoCache = noCache
	client.Refresh = refresh
	return client, nil
}

var parlCmd = &cobra.Command{
	Use:   "parl",
	Short: "Swiss Parliament open data (OData API)",
	Long:  "Query the Swiss Parliament OData API at ws.parlament.ch.",
}

// --- parl tables ---

var parlTablesCmd = &cobra.Command{
	Use:   "tables",
	Short: "List all available OData entity sets",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		tables, err := client.ParlTables()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.IsInteractive() {
			output.Section("Parliament API Entity Sets")
			rows := make([][]string, len(tables))
			for i, t := range tables {
				rows[i] = []string{t}
			}
			output.Table([]string{"EntitySet"}, rows)
		} else {
			output.JSON(tables)
		}
		return nil
	},
}

// --- parl schema ---

var parlSchemaCmd = &cobra.Command{
	Use:   "schema <table>",
	Short: "Show columns (properties) for an entity type",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		props, err := client.ParlSchema(args[0])
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.IsInteractive() {
			output.Section(fmt.Sprintf("Schema: %s", args[0]))
			rows := make([][]string, len(props))
			for i, p := range props {
				nullable := "yes"
				if p.Nullable == "false" {
					nullable = "no"
				}
				rows[i] = []string{p.Name, p.Type, nullable}
			}
			output.Table([]string{"Property", "Type", "Nullable"}, rows)
		} else {
			output.JSON(props)
		}
		return nil
	},
}

// --- parl query ---

var (
	parlQueryFilter  string
	parlQuerySelect  string
	parlQueryTop     int
	parlQuerySkip    int
	parlQueryOrderBy string
)

var parlQueryCmd = &cobra.Command{
	Use:   "query <table>",
	Short: "Generic OData query (always JSON output)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		q := api.NewODataQuery(args[0])
		if parlQueryFilter != "" {
			q.Filter(parlQueryFilter)
		}
		if parlQuerySelect != "" {
			q.Select(strings.Split(parlQuerySelect, ",")...)
		}
		if parlQueryTop > 0 {
			q.Top(parlQueryTop)
		}
		if parlQuerySkip > 0 {
			q.Skip(parlQuerySkip)
		}
		if parlQueryOrderBy != "" {
			q.OrderBy(parlQueryOrderBy)
		}

		raw, err := client.ParlQuery(q)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		// Always JSON for generic query.
		var data any
		if err := json.Unmarshal(raw, &data); err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}
		output.JSON(data)
		return nil
	},
}

// --- parl person (E: activity explorer when single match) ---

var (
	parlPersonName   string
	parlPersonParty  string
	parlPersonID     int
	parlPersonAll    bool // show all (active + inactive), default only active
)

var parlPersonCmd = &cobra.Command{
	Use:   "person",
	Short: "Search parliament members (shows activity profile for single match)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		q := api.NewODataQuery("MemberCouncil").
			Top(50).
			OrderBy("LastName")

		if !parlPersonAll && parlPersonID == 0 {
			q.Filter("Active eq true")
		}
		if parlPersonID > 0 {
			q.Filter(fmt.Sprintf("PersonNumber eq %d", parlPersonID))
		}
		if parlPersonName != "" {
			parts := strings.Fields(parlPersonName)
			if len(parts) >= 2 {
				// "First Last" → search both fields
				first := parts[0]
				last := strings.Join(parts[1:], " ")
				q.Filter(fmt.Sprintf("substringof('%s', FirstName) eq true and substringof('%s', LastName) eq true", first, last))
			} else {
				// Single term → search in LastName
				q.Filter(fmt.Sprintf("substringof('%s', LastName) eq true", parlPersonName))
			}
		}
		if parlPersonParty != "" {
			q.Filter(fmt.Sprintf("PartyAbbreviation eq '%s'", parlPersonParty))
		}

		var members []api.ParlMemberCouncil
		if err := client.ParlQueryInto(q, &members); err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if len(members) == 0 {
			output.Error("no members found")
			os.Exit(1)
		}

		// Feature E: if exactly 1 match, show full activity profile
		if len(members) == 1 {
			return showPersonProfile(client, members[0])
		}

		// Multiple matches: show list
		if output.IsInteractive() {
			output.Section("Parliament Members")
			rows := make([][]string, len(members))
			for i, m := range members {
				rows[i] = []string{
					fmt.Sprintf("%d", m.PersonNumber),
					m.FirstName + " " + m.LastName,
					api.Str(m.PartyAbbreviation),
					api.Str(m.CantonName),
					api.ParseODataDate(api.Str(m.DateJoining)),
				}
			}
			output.Table([]string{"ID", "Name", "Party", "Canton", "Joined"}, rows)
			fmt.Fprintf(os.Stderr, "\nTip: use --id <ID> to see a person's full activity profile\n")
		} else {
			output.JSON(members)
		}
		return nil
	},
}

func showPersonProfile(client *api.Client, m api.ParlMemberCouncil) error {
	// Fetch extended person details (birthdate, title, etc.)
	var persons []api.ParlPersonDetail
	pq := api.NewODataQuery("Person").
		Filter(fmt.Sprintf("PersonNumber eq %d", m.PersonNumber)).
		Top(1)
	client.ParlQueryInto(pq, &persons)

	// Fetch contact info
	var comms []api.ParlPersonCommunication
	commQ := api.NewODataQuery("PersonCommunication").
		Filter(fmt.Sprintf("PersonNumber eq %d", m.PersonNumber)).
		Top(10)
	client.ParlQueryInto(commQ, &comms)

	// Fetch committees
	var committees []api.ParlMemberCommittee
	cq := api.NewODataQuery("MemberCommittee").
		Filter(fmt.Sprintf("PersonNumber eq %d", m.PersonNumber)).
		Top(50)
	client.ParlQueryInto(cq, &committees)

	// Fetch business roles
	var roles []api.ParlBusinessRole
	rq := api.NewODataQuery("BusinessRole").
		Filter(fmt.Sprintf("MemberCouncilNumber eq %d", m.PersonNumber)).
		Top(20).
		OrderBy("BusinessSubmissionDate desc")
	client.ParlQueryInto(rq, &roles)

	// Fetch declared interests from Parliament API
	var parlInterests []api.ParlPersonInterest
	iq := api.NewODataQuery("PersonInterest").
		Filter(fmt.Sprintf("PersonNumber eq %d", m.PersonNumber)).
		Top(50)
	client.ParlQueryInto(iq, &parlInterests)

	// Fetch OpenParlData interests and lobby badges
	openInterests, _ := client.OpenParlInterests(m.PersonNumber)
	badges, _ := client.OpenParlAccessBadges(m.PersonNumber)

	if !output.IsInteractive() {
		profile := map[string]any{
			"person":           m,
			"details":          persons,
			"contact":          comms,
			"committees":       committees,
			"business":         roles,
			"interests_parl":   parlInterests,
			"interests_openparl": openInterests,
			"lobby_badges":     badges,
		}
		output.JSON(profile)
		return nil
	}

	// === Interactive profile ===
	active := "yes"
	if !m.Active {
		active = "no"
	}

	output.Section(fmt.Sprintf("%s %s (%s, %s)",
		m.FirstName, m.LastName, api.Str(m.PartyAbbreviation), api.Str(m.CantonAbbreviation)))

	// Basic info line
	fmt.Printf("Council: %s | Active: %s\n", api.Str(m.CouncilName), active)
	fmt.Printf("Joined: %s", api.ParseODataDate(api.Str(m.DateJoining)))
	if api.Str(m.DateLeaving) != "" {
		fmt.Printf(" | Left: %s", api.ParseODataDate(api.Str(m.DateLeaving)))
	}
	fmt.Println()

	// Extended person info (F)
	if len(persons) > 0 {
		p := persons[0]
		if api.Str(p.DateOfBirth) != "" {
			bdate := api.ParseODataDate(api.Str(p.DateOfBirth))
			fmt.Printf("Born: %s", bdate)
			if api.Str(p.PlaceOfBirthCity) != "" {
				fmt.Printf(" in %s", api.Str(p.PlaceOfBirthCity))
			}
			fmt.Println()
		}
		if api.Str(p.DateOfDeath) != "" {
			fmt.Printf("Died: %s\n", api.ParseODataDate(api.Str(p.DateOfDeath)))
		}
		if api.Str(p.TitleText) != "" {
			fmt.Printf("Title: %s\n", api.Str(p.TitleText))
		}
		if api.Str(p.MaritalStatusText) != "" {
			fmt.Printf("Marital status: %s\n", api.Str(p.MaritalStatusText))
		}
		if api.Int(p.NumberOfChildren) > 0 {
			fmt.Printf("Children: %d\n", api.Int(p.NumberOfChildren))
		}
	}

	// Contact info
	for _, c := range comms {
		fmt.Printf("%s: %s\n", api.Str(c.CommunicationTypeText), api.Str(c.Address))
	}

	// Committees
	if len(committees) > 0 {
		output.Section(fmt.Sprintf("Committee Memberships (%d)", len(committees)))
		rows := make([][]string, len(committees))
		for i, c := range committees {
			rows[i] = []string{
				api.Str(c.Abbreviation1),
				output.Truncate(api.Str(c.CommitteeName), 55),
				api.Str(c.CommitteeFunctionName),
			}
		}
		output.Table([]string{"Abbr", "Committee", "Function"}, rows)
	}

	// Business
	if len(roles) > 0 {
		output.Section(fmt.Sprintf("Recent Business (%d)", len(roles)))
		rows := make([][]string, len(roles))
		for i, r := range roles {
			rows[i] = []string{
				api.Str(r.BusinessShortNumber),
				api.Str(r.BusinessTypeAbbreviation),
				output.Truncate(api.Str(r.BusinessTitle), 45),
				api.Str(r.RoleName),
			}
		}
		output.Table([]string{"Number", "Type", "Title", "Role"}, rows)
	}

	// Declared interests (Parliament API)
	if len(parlInterests) > 0 {
		output.Section(fmt.Sprintf("Declared Interests (%d)", len(parlInterests)))
		rows := make([][]string, len(parlInterests))
		for i, interest := range parlInterests {
			rows[i] = []string{
				api.Str(interest.InterestName),
				api.Str(interest.FunctionInAgencyText),
				api.Str(interest.InterestTypeText),
			}
		}
		output.Table([]string{"Organization", "Function", "Type"}, rows)
	}

	// OpenParlData interests (G1)
	if len(openInterests) > 0 {
		output.Section(fmt.Sprintf("Corporate Roles — OpenParlData (%d)", len(openInterests)))
		rows := make([][]string, len(openInterests))
		for i, interest := range openInterests {
			rows[i] = []string{
				output.Truncate(interest.Name.Pick(output.Lang), 70),
			}
		}
		output.Table([]string{"Interest / Role"}, rows)
	}

	// Lobby badges (G1)
	if len(badges) > 0 {
		output.Section(fmt.Sprintf("Lobby Access Badges (%d)", len(badges)))
		rows := make([][]string, len(badges))
		for i, b := range badges {
			beneficiary := api.Str(b.BeneficiaryPersonFullname)
			group := api.Str(b.BeneficiaryGroup)
			typeStr := b.Type.Pick(output.Lang)
			if typeStr == "" {
				typeStr = api.Str(b.TypeHarmonized)
			}
			rows[i] = []string{
				beneficiary,
				group,
				typeStr,
				api.Str(b.ValidFrom) + " – " + api.Str(b.ValidTo),
			}
		}
		output.Table([]string{"Beneficiary", "Organization", "Type", "Valid"}, rows)
		fmt.Println("  Source: OpenParlData.ch")
	}

	return nil
}

// --- parl business ---

var (
	parlBusinessTitle string
	parlBusinessType  string
	parlBusinessBy    string
)

var parlBusinessCmd = &cobra.Command{
	Use:   "business [short-number]",
	Short: "Search or view parliamentary business items",
	Long: `Search for parliamentary business items, or view details for a specific item.

Examples:
  chli parl business                    # list recent items
  chli parl business 26.409             # detail view for 26.409
  chli parl business --title CO2        # search by title
  chli parl business --type Mo          # filter by type
  chli parl business --by "Franziska Roth"  # by submitter`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		q := api.NewODataQuery("Business").
			Top(50).
			OrderBy("SubmissionDate desc")

		// J: filter by short number if provided as positional arg
		if len(args) == 1 {
			q.Filter(fmt.Sprintf("BusinessShortNumber eq '%s'", args[0]))
		}

		if parlBusinessTitle != "" {
			q.Filter(fmt.Sprintf("substringof('%s', Title) eq true", parlBusinessTitle))
		}
		if parlBusinessType != "" {
			q.Filter(fmt.Sprintf("BusinessTypeAbbreviation eq '%s'", parlBusinessType))
		}
		if parlBusinessBy != "" {
			// SubmittedBy is "Last First" format — match each word separately
			parts := strings.Fields(parlBusinessBy)
			for _, part := range parts {
				q.Filter(fmt.Sprintf("substringof('%s', SubmittedBy) eq true", part))
			}
		}

		var items []api.ParlBusiness
		if err := client.ParlQueryInto(q, &items); err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if len(items) == 0 {
			if len(args) == 1 {
				output.Error("no business found with number " + args[0])
			} else {
				output.Error("no business items found")
			}
			os.Exit(1)
		}

		// K: single result → show detail view
		if len(items) == 1 {
			return showBusinessDetail(client, items[0])
		}

		// Multiple results → list view
		if output.IsInteractive() {
			output.Section("Parliamentary Business")
			rows := make([][]string, len(items))
			for i, b := range items {
				rows[i] = []string{
					b.BusinessShortNumber,
					b.BusinessTypeAbbreviation,
					output.Truncate(b.Title, 60),
					b.BusinessStatusText,
					api.ParseODataDate(api.Str(b.SubmissionDate)),
				}
			}
			output.Table([]string{"ShortNumber", "Type", "Title", "Status", "Date"}, rows)
		} else {
			output.JSON(items)
		}
		return nil
	},
}

func showBusinessDetail(client *api.Client, b api.ParlBusiness) error {
	// Fetch participants via BusinessRole
	var roles []api.ParlBusinessRole
	rq := api.NewODataQuery("BusinessRole").
		Filter(fmt.Sprintf("BusinessShortNumber eq '%s'", b.BusinessShortNumber)).
		Top(50)
	client.ParlQueryInto(rq, &roles)

	// Fetch status timeline
	var statuses []api.ParlBusinessStatusEntry
	sq := api.NewODataQuery("BusinessStatus").
		Filter(fmt.Sprintf("BusinessNumber eq %d", b.ID)).
		Top(20)
	client.ParlQueryInto(sq, &statuses)

	// Fetch preconsultations (committee assignments)
	var precons []api.ParlPreconsultation
	pcq := api.NewODataQuery("Preconsultation").
		Filter(fmt.Sprintf("BusinessNumber eq %d", b.ID)).
		Top(20)
	client.ParlQueryInto(pcq, &precons)

	// Fetch publications (external documents)
	var pubs []api.ParlPublication
	pubq := api.NewODataQuery("Publication").
		Filter(fmt.Sprintf("BusinessNumber eq %d", b.ID)).
		Top(20)
	client.ParlQueryInto(pubq, &pubs)

	// Fetch transcripts via SubjectBusiness → Transcript chain
	var subjects []api.ParlSubjectBusiness
	subq := api.NewODataQuery("SubjectBusiness").
		Filter(fmt.Sprintf("BusinessNumber eq %d", b.ID)).
		Top(10)
	client.ParlQueryInto(subq, &subjects)

	var transcripts []api.ParlTranscript
	for _, subj := range subjects {
		var trans []api.ParlTranscript
		tq := api.NewODataQuery("Transcript").
			Filter(fmt.Sprintf("IdSubject eq %s", subj.IdSubject)).
			Top(20)
		if err := client.ParlQueryInto(tq, &trans); err == nil {
			transcripts = append(transcripts, trans...)
		}
	}

	if !output.IsInteractive() {
		detail := map[string]any{
			"business":         b,
			"participants":     roles,
			"statuses":         statuses,
			"preconsultations": precons,
			"publications":     pubs,
			"transcripts":      transcripts,
		}
		output.JSON(detail)
		return nil
	}

	// === Interactive detail view ===
	output.Section(fmt.Sprintf("%s — %s", b.BusinessShortNumber, b.BusinessTypeAbbreviation))
	fmt.Println(b.Title)
	fmt.Printf("Status: %s | Submitted: %s\n", b.BusinessStatusText, api.ParseODataDate(api.Str(b.SubmissionDate)))
	if api.Str(b.SubmittedBy) != "" {
		fmt.Printf("Submitted by: %s\n", api.Str(b.SubmittedBy))
	}
	if api.Str(b.SubmissionCouncilName) != "" {
		fmt.Printf("Council: %s\n", api.Str(b.SubmissionCouncilName))
	}
	if api.Str(b.ResponsibleDepartmentName) != "" {
		fmt.Printf("Responsible: %s (%s)\n", api.Str(b.ResponsibleDepartmentName), api.Str(b.ResponsibleDepartmentAbbreviation))
	}
	if api.Str(b.TagNames) != "" {
		fmt.Printf("Tags: %s\n", api.Str(b.TagNames))
	}

	// Submitted text / description
	if text := api.Str(b.SubmittedText); text != "" {
		output.Section("Submitted Text")
		fmt.Println(stripHTML(text))
	}
	if text := api.Str(b.ReasonText); text != "" {
		output.Section("Reason")
		fmt.Println(stripHTML(text))
	}
	if text := api.Str(b.Description); text != "" {
		output.Section("Description")
		fmt.Println(stripHTML(text))
	}

	// Federal Council response
	if text := api.Str(b.FederalCouncilResponseText); text != "" {
		output.Section("Federal Council Response")
		if api.Str(b.FederalCouncilProposalText) != "" {
			fmt.Printf("Proposal: %s", api.Str(b.FederalCouncilProposalText))
			if api.Str(b.FederalCouncilProposalDate) != "" {
				fmt.Printf(" (%s)", api.ParseODataDate(api.Str(b.FederalCouncilProposalDate)))
			}
			fmt.Println()
		}
		fmt.Println(stripHTML(text))
	}

	// Participants — resolve names
	if len(roles) > 0 {
		// Collect unique MemberCouncilNumbers and resolve names
		nameMap := make(map[int]string)
		for _, r := range roles {
			mcn := api.Int(r.MemberCouncilNumber)
			if mcn > 0 {
				nameMap[mcn] = "" // placeholder
			}
		}
		// Batch resolve: build OR filter for MemberCouncil
		if len(nameMap) <= 20 {
			var filters []string
			for mcn := range nameMap {
				filters = append(filters, fmt.Sprintf("PersonNumber eq %d", mcn))
			}
			mq := api.NewODataQuery("MemberCouncil").
				Filter("(" + strings.Join(filters, " or ") + ")").
				Top(50)
			var members []api.ParlMemberCouncil
			if err := client.ParlQueryInto(mq, &members); err == nil {
				for _, m := range members {
					nameMap[m.PersonNumber] = m.FirstName + " " + m.LastName
				}
			}
		}

		resolveName := func(mcn int) string {
			if n, ok := nameMap[mcn]; ok && n != "" {
				return n
			}
			return fmt.Sprintf("#%d", mcn)
		}

		authors := []string{}
		cosigners := []string{}
		others := []string{}
		for _, r := range roles {
			mcn := api.Int(r.MemberCouncilNumber)
			name := resolveName(mcn)
			role := api.Str(r.RoleName)
			switch api.Int(r.Role) {
			case 7: // Urheber
				authors = append(authors, name)
			case 3: // Mitunterzeichner
				cosigners = append(cosigners, name)
			default:
				others = append(others, fmt.Sprintf("  %s (%s)", name, role))
			}
		}
		output.Section(fmt.Sprintf("Participants (%d)", len(roles)))
		if len(authors) > 0 {
			fmt.Printf("Author(s): %s\n", strings.Join(authors, ", "))
		}
		if len(cosigners) > 0 {
			fmt.Printf("Co-signers (%d): %s\n", len(cosigners), strings.Join(cosigners, ", "))
		}
		for _, o := range others {
			fmt.Println(o)
		}
	}

	// Status timeline
	if len(statuses) > 0 {
		output.Section("Status Timeline")
		rows := make([][]string, len(statuses))
		for i, s := range statuses {
			rows[i] = []string{
				api.ParseODataDate(api.Str(s.BusinessStatusDate)),
				api.Str(s.BusinessStatusName),
			}
		}
		output.Table([]string{"Date", "Status"}, rows)
	}

	// Committee assignments (preconsultations)
	if len(precons) > 0 {
		output.Section("Committee Assignments")
		rows := make([][]string, len(precons))
		for i, p := range precons {
			rows[i] = []string{
				api.Str(p.Abbreviation1),
				output.Truncate(api.Str(p.CommitteeName), 55),
				api.ParseODataDate(api.Str(p.PreconsultationDate)),
			}
		}
		output.Table([]string{"Committee", "Name", "Date"}, rows)
	}

	// Publications / documents
	if len(pubs) > 0 {
		output.Section("Publications / Documents")
		rows := make([][]string, len(pubs))
		for i, p := range pubs {
			rows[i] = []string{
				api.Str(p.PublicationTypeName),
				output.Truncate(api.Str(p.Title), 60),
				fmt.Sprintf("%d", api.Int(p.Year)),
			}
		}
		output.Table([]string{"Type", "Title", "Year"}, rows)
	}

	// Debate transcripts
	if len(transcripts) > 0 {
		// Resolve speaker names
		speakerIDs := make(map[int]bool)
		for _, t := range transcripts {
			if pn := api.Int(t.PersonNumber); pn > 0 {
				speakerIDs[pn] = true
			}
		}
		speakerNames := make(map[int]string)
		if len(speakerIDs) > 0 && len(speakerIDs) <= 20 {
			var filters []string
			for pn := range speakerIDs {
				filters = append(filters, fmt.Sprintf("PersonNumber eq %d", pn))
			}
			pq := api.NewODataQuery("Person").
				Filter("(" + strings.Join(filters, " or ") + ")").
				Top(50)
			var persons []api.ParlPersonDetail
			if err := client.ParlQueryInto(pq, &persons); err == nil {
				for _, p := range persons {
					speakerNames[p.PersonNumber] = p.FirstName + " " + p.LastName
				}
			}
		}

		output.Section("Debate Transcript")
		for _, t := range transcripts {
			text := stripHTML(api.Str(t.Text))
			if text == "" {
				continue
			}
			pn := api.Int(t.PersonNumber)
			speaker := api.Str(t.DisplayName)
			if speaker == "" {
				if name, ok := speakerNames[pn]; ok {
					speaker = name
				} else if pn > 0 {
					speaker = fmt.Sprintf("#%d", pn)
				}
			}
			fn := api.Str(t.Function)
			if speaker != "" {
				if fn != "" {
					fmt.Printf("[%s (%s)]:\n", speaker, fn)
				} else {
					fmt.Printf("[%s]:\n", speaker)
				}
			}
			fmt.Println(text)
			fmt.Println()
		}
	}

	// Link to parliament website
	fmt.Printf("\nDetails: https://www.parlament.ch/de/ratsbetrieb/suche-curia-vista/geschaeft?AffairId=%d\n", b.ID)

	return nil
}

// stripHTML removes HTML tags from a string for terminal display.
func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// --- parl vote (C: smart suggestions when no arg) ---

var parlVoteCmd = &cobra.Command{
	Use:   "vote [business-short-number]",
	Short: "Show voting results for a business item",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if len(args) == 0 {
			// Feature C: show recent votes as suggestions
			q := api.NewODataQuery("Vote").
				Top(10).
				OrderBy("RegistrationNumber desc")
			var recent []api.ParlVote
			if err := client.ParlQueryInto(q, &recent); err != nil {
				output.Error(err.Error())
				os.Exit(1)
			}
			seen := make(map[string]bool)
			fmt.Fprintln(os.Stderr, "No business number provided. Recent items with votes:")
			fmt.Fprintln(os.Stderr)
			for _, v := range recent {
				sn := api.Str(v.BusinessShortNumber)
				if sn == "" || seen[sn] {
					continue
				}
				seen[sn] = true
				fmt.Fprintf(os.Stderr, "  %s   %s\n", sn, output.Truncate(api.Str(v.BusinessTitle), 60))
			}
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Usage: chli parl vote <business-short-number>")
			os.Exit(1)
		}

		q := api.NewODataQuery("Vote").
			Filter(fmt.Sprintf("BusinessShortNumber eq '%s'", args[0])).
			Top(50).
			OrderBy("RegistrationNumber desc")

		var votes []api.ParlVote
		if err := client.ParlQueryInto(q, &votes); err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if len(votes) == 0 {
			output.Error(fmt.Sprintf("no votes found for business %s", args[0]))
			os.Exit(1)
		}

		if output.IsInteractive() {
			output.Section(fmt.Sprintf("Votes for %s", args[0]))
			rows := make([][]string, len(votes))
			for i, v := range votes {
				rows[i] = []string{
					fmt.Sprintf("%d", v.ID),
					output.Truncate(api.Str(v.Subject), 50),
					api.Str(v.MeaningYes),
					api.Str(v.MeaningNo),
					api.ParseODataDate(api.Str(v.VoteEnd)),
				}
			}
			output.Table([]string{"ID", "Subject", "Yes=", "No=", "Date"}, rows)
		} else {
			output.JSON(votes)
		}
		return nil
	},
}

// --- parl session ---

var parlSessionCurrent bool

var parlSessionCmd = &cobra.Command{
	Use:   "session",
	Short: "List parliamentary sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		q := api.NewODataQuery("Session").
			Top(20).
			OrderBy("StartDate desc")

		if parlSessionCurrent {
			q.Top(1)
		}

		var sessions []api.ParlSession
		if err := client.ParlQueryInto(q, &sessions); err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.IsInteractive() {
			output.Section("Parliamentary Sessions")
			rows := make([][]string, len(sessions))
			for i, s := range sessions {
				rows[i] = []string{
					fmt.Sprintf("%d", s.ID),
					api.Str(s.Code),
					api.Str(s.Title),
					api.ParseODataDate(api.Str(s.StartDate)),
					api.ParseODataDate(api.Str(s.EndDate)),
				}
			}
			output.Table([]string{"ID", "Code", "Title", "Start", "End"}, rows)
		} else {
			output.JSON(sessions)
		}
		return nil
	},
}

// --- parl committee ---

var parlCommitteeCmd = &cobra.Command{
	Use:   "committee",
	Short: "List parliamentary committees",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		q := api.NewODataQuery("Committee").
			Top(50).
			OrderBy("Name")

		var committees []api.ParlCommittee
		if err := client.ParlQueryInto(q, &committees); err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.IsInteractive() {
			output.Section("Parliamentary Committees")
			rows := make([][]string, len(committees))
			for i, c := range committees {
				rows[i] = []string{
					fmt.Sprintf("%d", c.ID),
					api.Str(c.Abbreviation),
					output.Truncate(api.Str(c.Name), 60),
					api.Str(c.CouncilName),
				}
			}
			output.Table([]string{"ID", "Abbr", "Name", "Council"}, rows)
		} else {
			output.JSON(committees)
		}
		return nil
	},
}

// --- parl summary (B: recent events digest) ---

var (
	parlSummaryPeriod string
	parlSummaryFrom   string
	parlSummaryTo     string
)

var parlSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Summary of recent parliamentary activity",
	Long: `Show a digest of recent parliamentary events: new business, votes, and sessions.

Period shortcuts: today (default), week, month, "last 30 days"
Or use --from / --to for explicit date ranges (YYYY-MM-DD).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		var since time.Time
		var until time.Time

		if parlSummaryFrom != "" {
			t, err := api.ParsePeriod(parlSummaryFrom)
			if err != nil {
				output.Error(err.Error())
				os.Exit(1)
			}
			since = t
		} else {
			t, err := api.ParsePeriod(parlSummaryPeriod)
			if err != nil {
				output.Error(err.Error())
				os.Exit(1)
			}
			since = t
		}

		if parlSummaryTo != "" {
			t, err := api.ParsePeriod(parlSummaryTo)
			if err != nil {
				output.Error(err.Error())
				os.Exit(1)
			}
			until = t
		}

		dateFilter := api.ODataDateTimeFilter("Modified", "ge", since)
		if !until.IsZero() {
			dateFilter += " and " + api.ODataDateTimeFilter("Modified", "le", until)
		}

		// Fetch business, votes, sessions in sequence (curl-based, can't parallelize)
		var business []api.ParlBusiness
		bq := api.NewODataQuery("Business").
			Filter(dateFilter).
			Top(20).
			OrderBy("SubmissionDate desc")
		client.ParlQueryInto(bq, &business)

		// Vote doesn't have Modified — filter by VoteEnd instead
		voteDateFilter := api.ODataDateTimeFilter("VoteEnd", "ge", since)
		if !until.IsZero() {
			voteDateFilter += " and " + api.ODataDateTimeFilter("VoteEnd", "le", until)
		}
		var votes []api.ParlVote
		vq := api.NewODataQuery("Vote").
			Filter(voteDateFilter).
			Top(20).
			OrderBy("RegistrationNumber desc")
		client.ParlQueryInto(vq, &votes)

		var sessions []api.ParlSession
		sq := api.NewODataQuery("Session").
			Top(3).
			OrderBy("StartDate desc")
		client.ParlQueryInto(sq, &sessions)

		if !output.IsInteractive() {
			output.JSON(map[string]any{
				"period":   parlSummaryPeriod,
				"since":    since.Format("2006-01-02"),
				"business": business,
				"votes":    votes,
				"sessions": sessions,
			})
			return nil
		}

		periodLabel := parlSummaryPeriod
		if parlSummaryFrom != "" {
			periodLabel = parlSummaryFrom
			if parlSummaryTo != "" {
				periodLabel += " to " + parlSummaryTo
			}
		}
		fmt.Printf("Parliamentary activity since %s (%s)\n", since.Format("2006-01-02"), periodLabel)

		if len(business) > 0 {
			output.Section(fmt.Sprintf("New Business (%d)", len(business)))
			rows := make([][]string, len(business))
			for i, b := range business {
				rows[i] = []string{
					b.BusinessShortNumber,
					b.BusinessTypeAbbreviation,
					output.Truncate(b.Title, 50),
					b.BusinessStatusText,
					api.ParseODataDate(api.Str(b.SubmissionDate)),
				}
			}
			output.Table([]string{"Number", "Type", "Title", "Status", "Date"}, rows)
		} else {
			fmt.Println("\nNo new business items.")
		}

		if len(votes) > 0 {
			// Deduplicate by BusinessShortNumber
			seen := make(map[string]bool)
			output.Section(fmt.Sprintf("Votes (%d)", len(votes)))
			var rows [][]string
			for _, v := range votes {
				sn := api.Str(v.BusinessShortNumber)
				if seen[sn] {
					continue
				}
				seen[sn] = true
				rows = append(rows, []string{
					sn,
					output.Truncate(api.Str(v.Subject), 45),
					api.Str(v.MeaningYes),
					api.ParseODataDate(api.Str(v.VoteEnd)),
				})
			}
			output.Table([]string{"Business", "Subject", "Yes=", "Date"}, rows)
		} else {
			fmt.Println("\nNo votes in this period.")
		}

		if len(sessions) > 0 {
			output.Section("Sessions")
			rows := make([][]string, len(sessions))
			for i, s := range sessions {
				rows[i] = []string{
					api.Str(s.Abbreviation),
					api.Str(s.Title),
					api.ParseODataDate(api.Str(s.StartDate)),
					api.ParseODataDate(api.Str(s.EndDate)),
				}
			}
			output.Table([]string{"Abbr", "Title", "Start", "End"}, rows)
		}

		return nil
	},
}

// --- parl interest (G2: reverse search by interest/company) ---

var parlInterestCmd = &cobra.Command{
	Use:   "interest <query>",
	Short: "Search for parliament members by interest or company name",
	Long:  "Find which parliament members are connected to a given company, organization, or interest.\nData source: OpenParlData.ch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		interests, err := client.OpenParlSearchInterests(args[0], 50)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if len(interests) == 0 {
			output.Error("no interests found matching: " + args[0])
			os.Exit(1)
		}

		if output.IsInteractive() {
			output.Section(fmt.Sprintf("Interests matching %q (%d)", args[0], len(interests)))
			rows := make([][]string, len(interests))
			for i, interest := range interests {
				rows[i] = []string{
					interest.PersonFullname,
					output.Truncate(interest.Name.Pick(output.Lang), 55),
					interest.RoleName.Pick(output.Lang),
				}
			}
			output.Table([]string{"Person", "Interest / Organization", "Role"}, rows)
			fmt.Println("  Source: OpenParlData.ch")
		} else {
			output.JSON(interests)
		}
		return nil
	},
}

// --- parl schema (C: smart suggestions when no arg) ---

func init() {
	// Override schema to show suggestions
	parlSchemaCmd.Args = cobra.MaximumNArgs(1)
	origSchemaRun := parlSchemaCmd.RunE
	parlSchemaCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "No table name provided. Common entity types:")
			fmt.Fprintln(os.Stderr)
			for _, t := range []string{"Business", "Person", "MemberCouncil", "Vote", "Voting", "Session", "Committee", "Party", "Canton"} {
				fmt.Fprintf(os.Stderr, "  %s\n", t)
			}
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Usage: chli parl schema <table>")
			fmt.Fprintln(os.Stderr, "Run 'chli parl tables' to see all available entity types.")
			os.Exit(1)
		}
		return origSchemaRun(cmd, args)
	}

	// query flags
	parlQueryCmd.Flags().StringVar(&parlQueryFilter, "filter", "", "OData $filter expression")
	parlQueryCmd.Flags().StringVar(&parlQuerySelect, "select", "", "Comma-separated $select fields")
	parlQueryCmd.Flags().IntVar(&parlQueryTop, "top", 20, "Max results ($top)")
	parlQueryCmd.Flags().IntVar(&parlQuerySkip, "skip", 0, "Skip N results ($skip)")
	parlQueryCmd.Flags().StringVar(&parlQueryOrderBy, "orderby", "", "OData $orderby expression")

	// person flags
	parlPersonCmd.Flags().StringVar(&parlPersonName, "name", "", "Filter by name (supports \"First Last\")")
	parlPersonCmd.Flags().StringVar(&parlPersonParty, "party", "", "Filter by party abbreviation (e.g. SP, SVP, FDP)")
	parlPersonCmd.Flags().IntVar(&parlPersonID, "id", 0, "Look up by exact PersonNumber")
	parlPersonCmd.Flags().BoolVar(&parlPersonAll, "all", false, "Include inactive/retired members (default: active only)")

	// business flags
	parlBusinessCmd.Flags().StringVar(&parlBusinessTitle, "title", "", "Filter by title (substring)")
	parlBusinessCmd.Flags().StringVar(&parlBusinessType, "type", "", "Filter by business type abbreviation (e.g. Mo, Ip, Po)")
	parlBusinessCmd.Flags().StringVar(&parlBusinessBy, "by", "", "Filter by submitter name (substring, e.g. \"Franziska Roth\")")

	// session flags
	parlSessionCmd.Flags().BoolVar(&parlSessionCurrent, "current", false, "Show only the most recent session")

	// summary flags
	parlSummaryCmd.Flags().StringVar(&parlSummaryPeriod, "period", "today", `Time period: today, week, month, "last N days"`)
	parlSummaryCmd.Flags().StringVar(&parlSummaryFrom, "from", "", "Start date (YYYY-MM-DD)")
	parlSummaryCmd.Flags().StringVar(&parlSummaryTo, "to", "", "End date (YYYY-MM-DD)")

	// Wire up subcommands.
	parlCmd.AddCommand(parlTablesCmd)
	parlCmd.AddCommand(parlSchemaCmd)
	parlCmd.AddCommand(parlQueryCmd)
	parlCmd.AddCommand(parlPersonCmd)
	parlCmd.AddCommand(parlBusinessCmd)
	parlCmd.AddCommand(parlVoteCmd)
	parlCmd.AddCommand(parlSessionCmd)
	parlCmd.AddCommand(parlCommitteeCmd)
	parlCmd.AddCommand(parlSummaryCmd)
	parlCmd.AddCommand(parlInterestCmd)

	rootCmd.AddCommand(parlCmd)
}
