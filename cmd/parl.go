package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
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
	Run: func(cmd *cobra.Command, args []string) {
		printSessionOverview()
		_ = cmd.Help()
	},
}

// printSessionOverview prints the current session, or the most recent past
// and next upcoming session, to stderr before the help text.
func printSessionOverview() {
	if output.ForceJSON || output.OutputFormat == "json" ||
		output.OutputFormat == "csv" || output.OutputFormat == "tsv" {
		return
	}
	client, err := newParlClient()
	if err != nil {
		return
	}
	sessions, err := fetchSessionsAround(client, 20)
	if err != nil || len(sessions) == 0 {
		return
	}
	if cur := findCurrentSession(sessions); cur != nil {
		output.Section("Current Session")
		printSessionLine(*cur)
		fmt.Println()
		return
	}
	past, next := findAdjacentSessions(sessions)
	output.Section("Parliamentary Sessions")
	if past != nil {
		fmt.Print("Past:     ")
		printSessionLine(*past)
	}
	if next != nil {
		fmt.Print("Next:     ")
		printSessionLine(*next)
	} else if agendaNext := fetchNextAgendaSession(client); agendaNext != nil {
		// OData hasn't registered the next session yet — fall back to
		// the parlament.ch agenda search, which publishes future dates.
		fmt.Print("Next:     ")
		printSessionLine(agendaToSession(*agendaNext))
	}
	fmt.Println()
}

// fetchNextAgendaSession returns the soonest upcoming Session event from the
// parlament.ch agenda, or nil if the lookup fails or no upcoming session
// exists. Errors are swallowed: this is an optional enrichment.
func fetchNextAgendaSession(client *api.Client) *api.ParlAgendaEvent {
	events, err := client.FetchAgendaEvents("PdAgendaCategoryEN:Session", 20)
	if err != nil {
		return nil
	}
	now := time.Now().Truncate(24 * time.Hour)
	var best *api.ParlAgendaEvent
	for i := range events {
		e := events[i]
		if !strings.EqualFold(e.CategoryEn, "Session") {
			continue
		}
		end := e.EndEventDate
		if end.IsZero() {
			end = e.EventDate
		}
		if end.Before(now) {
			continue
		}
		if best == nil || e.EventDate.Before(best.EventDate) {
			best = &events[i]
		}
	}
	return best
}

func printSessionLine(s api.ParlSession) {
	name := api.Str(s.SessionName)
	if name == "" {
		name = api.Str(s.Title)
	}
	abbr := api.Str(s.Abbreviation)
	if abbr != "" {
		fmt.Printf("%s (%s) — %s to %s\n",
			name, abbr,
			api.ParseODataDate(api.Str(s.StartDate)),
			api.ParseODataDate(api.Str(s.EndDate)))
		return
	}
	fmt.Printf("%s — %s to %s\n",
		name,
		api.ParseODataDate(api.Str(s.StartDate)),
		api.ParseODataDate(api.Str(s.EndDate)))
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

	// Declared interests — merge Parliament API with OpenParlData extras.
	type interestRow struct {
		org, function, itype, paid string
	}
	var mergedInterests []interestRow
	seen := make(map[string]bool)

	// Parliament API entries are the primary source.
	for _, pi := range parlInterests {
		name := api.Str(pi.InterestName)
		paid := ""
		if pi.Paid != nil {
			if *pi.Paid {
				paid = "yes"
			} else {
				paid = "no"
			}
		}
		mergedInterests = append(mergedInterests, interestRow{
			org:      name,
			function: api.Str(pi.FunctionInAgencyText),
			itype:    api.Str(pi.InterestTypeText),
			paid:     paid,
		})
		seen[name] = true
	}

	// Add any OpenParlData entries not already covered by Parliament data.
	for _, oi := range openInterests {
		name := oi.Name.Pick(output.Lang)
		if seen[name] {
			continue
		}
		paid := ""
		if h := api.Str(oi.TypePaymentHarmonized); h == "honorary" {
			paid = "no"
		} else if h == "paid" {
			paid = "yes"
		}
		mergedInterests = append(mergedInterests, interestRow{
			org:      name,
			function: oi.RoleName.Pick(output.Lang),
			itype:    oi.Group.Pick(output.Lang),
			paid:     paid,
		})
	}

	if len(mergedInterests) > 0 {
		output.Section(fmt.Sprintf("Declared Interests (%d)", len(mergedInterests)))
		rows := make([][]string, len(mergedInterests))
		for i, r := range mergedInterests {
			rows[i] = []string{r.org, r.function, r.itype, r.paid}
		}
		output.Table([]string{"Organization", "Function", "Type", "Paid"}, rows)
	}

	// Lobby badges (G1) — deduplicate by beneficiary+org, keep latest ValidTo
	if len(badges) > 0 {
		type badgeKey struct{ beneficiary, group string }
		best := make(map[badgeKey]api.OpenParlAccessBadge)
		for _, b := range badges {
			k := badgeKey{api.Str(b.BeneficiaryPersonFullname), api.Str(b.BeneficiaryGroup)}
			if prev, ok := best[k]; !ok || api.Str(b.ValidTo) > api.Str(prev.ValidTo) {
				best[k] = b
			}
		}
		deduped := make([]api.OpenParlAccessBadge, 0, len(best))
		for _, b := range best {
			deduped = append(deduped, b)
		}
		output.Section(fmt.Sprintf("Lobby Access Badges (%d)", len(deduped)))
		rows := make([][]string, len(deduped))
		for i, b := range deduped {
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

var parlVoteDetail bool

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
				// Fetch vote counts from individual Voting records.
				counts, err := client.FetchVoteCounts(v.ID)
				var yesNo string
				if err == nil {
					yesNo = fmt.Sprintf("%d / %d / %d", counts.Yes, counts.No, counts.Abstain)
				}
				// Subject/MeaningYes/MeaningNo are not translated by the API (always French).
				// Translate them client-side.
				rows[i] = []string{
					fmt.Sprintf("%d", v.ID),
					output.Truncate(api.TranslateVoteText(api.Str(v.Subject)), 50),
					yesNo,
					api.TranslateVoteText(api.Str(v.MeaningYes)),
					api.TranslateVoteText(api.Str(v.MeaningNo)),
					api.ParseODataDate(api.Str(v.VoteEnd)),
				}
			}
			output.Table([]string{"ID", "Subject", "Y/N/A", "Yes=", "No=", "Date"}, rows)
		} else {
			type voteWithCounts struct {
				api.ParlVote
				Yes     int `json:"Yes"`
				No      int `json:"No"`
				Abstain int `json:"Abstain"`
				Absent  int `json:"Absent"`
			}
			enriched := make([]voteWithCounts, len(votes))
			for i, v := range votes {
				enriched[i].ParlVote = v
				if counts, err := client.FetchVoteCounts(v.ID); err == nil {
					enriched[i].Yes = counts.Yes
					enriched[i].No = counts.No
					enriched[i].Abstain = counts.Abstain
					enriched[i].Absent = counts.Absent
				}
			}
			output.JSON(enriched)
		}

		// --detail: show per-member and per-party breakdown for each vote
		if parlVoteDetail {
			for _, v := range votes {
				votings, err := client.FetchVotings(v.ID)
				if err != nil {
					output.Error(fmt.Sprintf("failed to fetch voting details for vote %d: %s", v.ID, err))
					continue
				}
				if len(votings) == 0 {
					continue
				}
				renderVoteDetail(v, votings)
			}
		}

		return nil
	},
}

// decisionChar returns a single character representing a vote decision.
func decisionChar(decision int) string {
	switch decision {
	case 1:
		return "Y"
	case 2:
		return "N"
	case 3:
		return "A"
	default:
		return "."
	}
}

// decisionLabel returns a human-readable label for a vote decision.
func decisionLabel(decision int) string {
	switch decision {
	case 1:
		return "Yes"
	case 2:
		return "No"
	case 3:
		return "Abstain"
	case 5:
		return "Not Participated"
	case 6:
		return "Excused"
	case 7:
		return "President"
	default:
		return "Absent"
	}
}

// colorDecision applies ANSI color to a decision character if interactive.
func colorDecision(ch string, decision int) string {
	if output.NoColor || !output.IsInteractive() {
		return ch
	}
	switch decision {
	case 1:
		return "\033[32m" + ch + "\033[0m" // green
	case 2:
		return "\033[31m" + ch + "\033[0m" // red
	case 3:
		return "\033[33m" + ch + "\033[0m" // yellow
	default:
		return "\033[90m" + ch + "\033[0m" // gray
	}
}

// partyBreakdown holds per-party vote counts.
type partyBreakdown struct {
	Party   string
	Yes     int
	No      int
	Abstain int
	Absent  int
	Total   int
}

// renderVoteDetail displays per-member breakdown, per-party breakdown,
// and an ASCII seat visualization for a single vote.
func renderVoteDetail(vote api.ParlVote, votings []api.ParlVoting) {
	subject := api.TranslateVoteText(api.Str(vote.Subject))
	fmt.Println()
	output.Section(fmt.Sprintf("Vote %d: %s", vote.ID, subject))

	// Aggregate counts
	var totalYes, totalNo, totalAbstain, totalAbsent int
	parties := make(map[string]*partyBreakdown)
	var partyOrder []string

	for _, v := range votings {
		d := api.Int(v.Decision)
		party := api.Str(v.ParlGroupName)
		if party == "" {
			party = "–"
		}

		if _, ok := parties[party]; !ok {
			parties[party] = &partyBreakdown{Party: party}
			partyOrder = append(partyOrder, party)
		}
		pb := parties[party]
		pb.Total++

		switch d {
		case 1:
			totalYes++
			pb.Yes++
		case 2:
			totalNo++
			pb.No++
		case 3:
			totalAbstain++
			pb.Abstain++
		default:
			totalAbsent++
			pb.Absent++
		}
	}

	// Sort parties by total members (descending)
	sort.Slice(partyOrder, func(i, j int) bool {
		return parties[partyOrder[i]].Total > parties[partyOrder[j]].Total
	})

	// Summary
	fmt.Printf("  Total: %d members\n", len(votings))
	fmt.Printf("  Yes: %d  No: %d  Abstain: %d  Absent: %d\n\n", totalYes, totalNo, totalAbstain, totalAbsent)

	// Per-party breakdown table
	if output.IsInteractive() {
		output.Section("Per-Party Breakdown")
		partyRows := make([][]string, len(partyOrder))
		for i, p := range partyOrder {
			pb := parties[p]
			partyRows[i] = []string{
				pb.Party,
				fmt.Sprintf("%d", pb.Yes),
				fmt.Sprintf("%d", pb.No),
				fmt.Sprintf("%d", pb.Abstain),
				fmt.Sprintf("%d", pb.Absent),
				fmt.Sprintf("%d", pb.Total),
			}
		}
		output.Table([]string{"Party", "Yes", "No", "Abstain", "Absent", "Total"}, partyRows)
	}

	// Per-member list
	if output.IsInteractive() {
		// Sort members by party then last name
		sorted := make([]api.ParlVoting, len(votings))
		copy(sorted, votings)
		sort.Slice(sorted, func(i, j int) bool {
			pi := api.Str(sorted[i].ParlGroupName)
			pj := api.Str(sorted[j].ParlGroupName)
			if pi != pj {
				return pi < pj
			}
			return sorted[i].LastName < sorted[j].LastName
		})

		output.Section("Per-Member Votes")
		memberRows := make([][]string, len(sorted))
		for i, v := range sorted {
			d := api.Int(v.Decision)
			label := decisionLabel(d)
			memberRows[i] = []string{
				v.LastName + ", " + v.FirstName,
				api.Str(v.ParlGroupName),
				api.Str(v.Canton),
				output.Highlight(label),
			}
		}
		output.Table([]string{"Name", "Party", "Canton", "Decision"}, memberRows)
	} else {
		// JSON mode: output structured detail
		type detailJSON struct {
			VoteID  int                `json:"VoteID"`
			Summary map[string]int     `json:"Summary"`
			Parties []partyBreakdown   `json:"Parties"`
			Members []api.ParlVoting   `json:"Members"`
		}
		pbs := make([]partyBreakdown, len(partyOrder))
		for i, p := range partyOrder {
			pbs[i] = *parties[p]
		}
		output.JSON(detailJSON{
			VoteID: vote.ID,
			Summary: map[string]int{
				"Yes": totalYes, "No": totalNo,
				"Abstain": totalAbstain, "Absent": totalAbsent,
			},
			Parties: pbs,
			Members: votings,
		})
	}

	// ASCII seat visualization
	if output.IsInteractive() {
		renderSeatVisualization(partyOrder, parties, votings)
	}
}

// renderSeatVisualization renders an ASCII visualization of seats grouped by party.
func renderSeatVisualization(partyOrder []string, parties map[string]*partyBreakdown, votings []api.ParlVoting) {
	fmt.Println()
	output.Section("Seat Visualization")
	fmt.Println("  Y=Yes  N=No  A=Abstain  .=Absent")
	fmt.Println()

	// Group votings by party
	byParty := make(map[string][]api.ParlVoting)
	for _, v := range votings {
		p := api.Str(v.ParlGroupName)
		if p == "" {
			p = "–"
		}
		byParty[p] = append(byParty[p], v)
	}

	for _, party := range partyOrder {
		members := byParty[party]
		if len(members) == 0 {
			continue
		}

		// Sort within party: Yes first, then No, Abstain, Absent
		sort.Slice(members, func(i, j int) bool {
			return api.Int(members[i].Decision) < api.Int(members[j].Decision)
		})

		fmt.Printf("  %s (%d):\n  ", party, len(members))
		for i, v := range members {
			d := api.Int(v.Decision)
			ch := decisionChar(d)
			fmt.Print(colorDecision(ch, d))
			// Add space every 8 seats for readability
			if (i+1)%8 == 0 && i+1 < len(members) {
				fmt.Print(" ")
			}
		}
		fmt.Println()
	}
	fmt.Println()
}

// --- parl events (upcoming agenda from parlament.ch SharePoint search) ---

var (
	parlEventsCategory string
	parlEventsSessions bool
	parlEventsLimit    int
	parlEventsAll      bool
)

var parlEventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Upcoming parliamentary events (sessions, press conferences, ceremonies)",
	Long: `List upcoming events from the parlament.ch agenda.

This queries the SharePoint search endpoint behind the public agenda page and
returns structured data (event dates, localized titles, category, location).
It complements 'parl session', which is limited to sessions already registered
in the OData Session entity — the agenda surfaces further-out scheduling.

Examples:
  chli parl events                      # all upcoming events
  chli parl events --sessions           # only upcoming sessions
  chli parl events --category Session   # same, long form
  chli parl events --all                # include past events too
  chli parl events --lang fr            # French titles and categories`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		category := parlEventsCategory
		if parlEventsSessions && category == "" {
			category = "Session"
		}

		query := ""
		if category != "" {
			// Quote values containing spaces for KQL.
			val := category
			if strings.ContainsAny(val, " \t") {
				val = `"` + strings.ReplaceAll(val, `"`, `\"`) + `"`
			}
			query = "PdAgendaCategoryEN:" + val
		}

		events, err := client.FetchAgendaEvents(query, parlEventsLimit)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		// KQL property filters occasionally leak non-matching results,
		// so also enforce the category client-side for exact matching.
		if category != "" {
			filtered := events[:0]
			for _, e := range events {
				if strings.EqualFold(e.CategoryEn, category) {
					filtered = append(filtered, e)
				}
			}
			events = filtered
		}

		if !parlEventsAll {
			cutoff := time.Now().Truncate(24 * time.Hour)
			filtered := events[:0]
			for _, e := range events {
				end := e.EndEventDate
				if end.IsZero() {
					end = e.EventDate
				}
				if !end.Before(cutoff) {
					filtered = append(filtered, e)
				}
			}
			events = filtered
		}

		// Sort ascending by EventDate for readability.
		sort.Slice(events, func(i, j int) bool {
			return events[i].EventDate.Before(events[j].EventDate)
		})

		if !output.IsInteractive() {
			output.JSON(events)
			return nil
		}

		output.Section("Parliamentary Agenda")
		rows := make([][]string, len(events))
		for i, e := range events {
			end := ""
			if !e.EndEventDate.IsZero() {
				end = e.EndEventDate.Format("2006-01-02")
			}
			rows[i] = []string{
				e.EventDate.Format("2006-01-02"),
				end,
				e.LocalizedCategory(output.Lang),
				output.Truncate(e.LocalizedTitle(output.Lang), 70),
			}
		}
		output.Table([]string{"Start", "End", "Category", "Title"}, rows)
		return nil
	},
}

// --- parl session ---

var parlSessionCurrent bool

var parlSessionCmd = &cobra.Command{
	Use:   "session [id]",
	Short: "List parliamentary sessions or view session agenda",
	Long: `List parliamentary sessions, or view agenda items for a specific session.

Examples:
  chli parl session                # list recent sessions
  chli parl session 5121           # show agenda for session 5121
  chli parl session --current      # show the most recent session`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		// If a session ID is given, show the agenda for that session
		if len(args) == 1 {
			return showSessionAgenda(client, args[0])
		}

		sessions, err := fetchSessionsAround(client, 20)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if parlSessionCurrent {
			if cur := findCurrentSession(sessions); cur != nil {
				sessions = []api.ParlSession{*cur}
			} else {
				sessions = nil
			}
		}

		if output.IsInteractive() {
			output.Section("Parliamentary Sessions")
			now := time.Now()
			rows := make([][]string, len(sessions))
			for i, s := range sessions {
				idStr := ""
				if s.ID != 0 {
					idStr = fmt.Sprintf("%d", s.ID)
				}
				rows[i] = []string{
					idStr,
					api.Str(s.Abbreviation),
					api.Str(s.SessionName),
					api.Str(s.Title),
					api.ParseODataDate(api.Str(s.StartDate)),
					api.ParseODataDate(api.Str(s.EndDate)),
					sessionStatus(s, now),
				}
			}
			output.Table([]string{"ID", "Abbr", "Name", "Title", "Start", "End", "Status"}, rows)
			fmt.Fprintf(os.Stderr, "\nTip: use 'chli parl session <ID>' to see the agenda\n")
		} else {
			output.JSON(sessions)
		}
		return nil
	},
}

// fetchSessionsAround returns sessions ordered by StartDate desc, including
// upcoming ones. n bounds the total count from OData; any future sessions
// announced on the parlament.ch agenda but not yet in OData are merged in
// on top (deduplicated by start date).
func fetchSessionsAround(client *api.Client, n int) ([]api.ParlSession, error) {
	q := api.NewODataQuery("Session").
		Top(n).
		OrderBy("StartDate desc")
	var sessions []api.ParlSession
	if err := client.ParlQueryInto(q, &sessions); err != nil {
		return nil, err
	}

	// Merge in future sessions from the agenda search. Failures here are
	// non-fatal — the OData list is still useful on its own.
	if extras, err := fetchAgendaSessions(client); err == nil {
		known := make(map[string]bool, len(sessions))
		for _, s := range sessions {
			known[api.ParseODataDate(api.Str(s.StartDate))] = true
		}
		for _, e := range extras {
			start := e.EventDate.Format("2006-01-02")
			if known[start] {
				continue
			}
			sessions = append(sessions, agendaToSession(e))
		}
		sort.Slice(sessions, func(i, j int) bool {
			return api.ParseODataDate(api.Str(sessions[i].StartDate)) >
				api.ParseODataDate(api.Str(sessions[j].StartDate))
		})
	}

	return sessions, nil
}

// fetchAgendaSessions returns future sessions announced on the parlament.ch
// agenda page (which publishes further ahead than the OData Session entity).
func fetchAgendaSessions(client *api.Client) ([]api.ParlAgendaEvent, error) {
	events, err := client.FetchAgendaEvents("PdAgendaCategoryEN:Session", 50)
	if err != nil {
		return nil, err
	}
	now := time.Now().Truncate(24 * time.Hour)
	out := events[:0]
	for _, e := range events {
		if !strings.EqualFold(e.CategoryEn, "Session") {
			continue
		}
		end := e.EndEventDate
		if end.IsZero() {
			end = e.EventDate
		}
		if end.Before(now) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// agendaSessionDateRE strips the explicit date range from an agenda session
// title. Example inputs and outputs (all four languages):
//
//	"Wintersession: 6. – 23. Dezember 2027"        -> "Wintersession 2027"
//	"Session d'hiver: 6 – 23 décembre 2027"         -> "Session d'hiver 2027"
//	"Sessione invernale: 6 – 23 dicembre 2027"      -> "Sessione invernale 2027"
//	"Winter session: 6 – 23 December 2027"          -> "Winter session 2027"
var agendaSessionDateRE = regexp.MustCompile(`:\s*.+?\s+(\d{4})\s*$`)

// cleanAgendaSessionTitle removes the trailing date range and keeps the year.
func cleanAgendaSessionTitle(title string) string {
	return agendaSessionDateRE.ReplaceAllString(title, " $1")
}

// agendaToSession synthesizes a ParlSession record from an agenda event so
// upcoming-but-not-yet-registered sessions surface in the normal list.
func agendaToSession(e api.ParlAgendaEvent) api.ParlSession {
	name := cleanAgendaSessionTitle(e.LocalizedTitle(output.Lang))
	startODataStr := fmt.Sprintf("/Date(%d)/", e.EventDate.UnixMilli())
	endODataStr := ""
	if !e.EndEventDate.IsZero() {
		endODataStr = fmt.Sprintf("/Date(%d)/", e.EndEventDate.UnixMilli())
	}
	start := &startODataStr
	var end *string
	if endODataStr != "" {
		end = &endODataStr
	}
	return api.ParlSession{
		SessionName: &name,
		StartDate:   start,
		EndDate:     end,
	}
}

func parseODataTime(s string) (time.Time, bool) {
	d := api.ParseODataDate(s)
	if d == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func sessionStatus(s api.ParlSession, now time.Time) string {
	start, okS := parseODataTime(api.Str(s.StartDate))
	end, okE := parseODataTime(api.Str(s.EndDate))
	if !okS || !okE {
		return ""
	}
	today := now.Truncate(24 * time.Hour)
	if !today.Before(start) && !today.After(end) {
		return "current"
	}
	if today.Before(start) {
		return "upcoming"
	}
	return "past"
}

func findCurrentSession(sessions []api.ParlSession) *api.ParlSession {
	now := time.Now()
	for i := range sessions {
		if sessionStatus(sessions[i], now) == "current" {
			return &sessions[i]
		}
	}
	return nil
}

// findAdjacentSessions returns the most recent past session and the next
// upcoming session, given sessions ordered by StartDate desc.
func findAdjacentSessions(sessions []api.ParlSession) (past, next *api.ParlSession) {
	now := time.Now()
	for i := range sessions {
		switch sessionStatus(sessions[i], now) {
		case "upcoming":
			next = &sessions[i]
		case "past":
			if past == nil {
				past = &sessions[i]
			}
		}
	}
	return past, next
}

func showSessionAgenda(client *api.Client, sessionIDStr string) error {
	sessionID := 0
	if _, err := fmt.Sscanf(sessionIDStr, "%d", &sessionID); err != nil {
		output.Error("invalid session ID: " + sessionIDStr)
		os.Exit(1)
	}

	// Fetch session info
	sq := api.NewODataQuery("Session").
		Filter(fmt.Sprintf("ID eq %d", sessionID)).
		Top(1)
	var sessions []api.ParlSession
	if err := client.ParlQueryInto(sq, &sessions); err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
	if len(sessions) == 0 {
		output.Error("session not found: " + sessionIDStr)
		os.Exit(1)
	}
	session := sessions[0]

	// Find business items discussed in this session via Vote entity
	vq := api.NewODataQuery("Vote").
		Filter(fmt.Sprintf("IdSession eq %d", sessionID)).
		Top(100).
		OrderBy("RegistrationNumber")
	var votes []api.ParlVote
	if err := client.ParlQueryInto(vq, &votes); err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}

	// Collect unique business short numbers from votes
	type businessInfo struct {
		ShortNumber string
		Title       string
	}
	seen := make(map[string]bool)
	var items []businessInfo
	for _, v := range votes {
		sn := api.Str(v.BusinessShortNumber)
		if sn == "" || seen[sn] {
			continue
		}
		seen[sn] = true
		items = append(items, businessInfo{
			ShortNumber: sn,
			Title:       api.Str(v.BusinessTitle),
		})
	}

	if !output.IsInteractive() {
		result := map[string]any{
			"session": session,
			"items":   items,
		}
		output.JSON(result)
		return nil
	}

	sessionLabel := api.Str(session.SessionName)
	if sessionLabel == "" {
		sessionLabel = api.Str(session.Title)
	}
	output.Section(fmt.Sprintf("Session %d: %s", session.ID, sessionLabel))
	fmt.Printf("%s | %s | %s to %s\n\n",
		api.Str(session.Abbreviation),
		api.Str(session.Title),
		api.ParseODataDate(api.Str(session.StartDate)),
		api.ParseODataDate(api.Str(session.EndDate)))

	if len(items) == 0 {
		fmt.Println("No business items with votes found for this session.")
		return nil
	}

	// Fetch full business details for these items to get type info
	var filters []string
	for _, item := range items {
		filters = append(filters, fmt.Sprintf("BusinessShortNumber eq '%s'", item.ShortNumber))
	}

	// Query in batches if needed (OData filter length limits)
	businessMap := make(map[string]api.ParlBusiness)
	batchSize := 10
	for i := 0; i < len(filters); i += batchSize {
		end := i + batchSize
		if end > len(filters) {
			end = len(filters)
		}
		bq := api.NewODataQuery("Business").
			Filter("(" + strings.Join(filters[i:end], " or ") + ")").
			Top(batchSize)
		var businesses []api.ParlBusiness
		if err := client.ParlQueryInto(bq, &businesses); err == nil {
			for _, b := range businesses {
				businessMap[b.BusinessShortNumber] = b
			}
		}
	}

	fmt.Printf("Business items discussed (%d):\n\n", len(items))
	rows := make([][]string, len(items))
	for i, item := range items {
		typeAbbr := ""
		title := item.Title
		if b, ok := businessMap[item.ShortNumber]; ok {
			typeAbbr = b.BusinessTypeAbbreviation
			if b.Title != "" {
				title = b.Title
			}
		}
		rows[i] = []string{
			item.ShortNumber,
			typeAbbr,
			output.Truncate(title, 65),
		}
	}
	output.Table([]string{"Number", "Type", "Title"}, rows)

	return nil
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
					output.Truncate(api.TranslateVoteText(api.Str(v.Subject)), 45),
					api.TranslateVoteText(api.Str(v.MeaningYes)),
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
					api.Str(s.SessionName),
					api.Str(s.Title),
					api.ParseODataDate(api.Str(s.StartDate)),
					api.ParseODataDate(api.Str(s.EndDate)),
				}
			}
			output.Table([]string{"Abbr", "Name", "Title", "Start", "End"}, rows)
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

// --- parl department (federal departments via legacy ws-old endpoint) ---

var parlDepartmentHistoric bool

var parlDepartmentCmd = &cobra.Command{
	Use:   "department",
	Short: "List federal departments (data source: ws-old.parlament.ch)",
	Long: `List federal departments. The current OData service at ws.parlament.ch does
not expose departments, so this command uses the legacy ws-old.parlament.ch
endpoint. Use --historic to include end-dated historic records.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newParlClient()
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		depts, err := client.ParlDepartments(parlDepartmentHistoric)
		if err != nil {
			output.Error(err.Error())
			os.Exit(1)
		}

		if output.IsInteractive() {
			if parlDepartmentHistoric {
				output.Section("Federal Departments (historic)")
				rows := make([][]string, len(depts))
				for i, d := range depts {
					rows[i] = []string{
						fmt.Sprintf("%d", d.ID),
						d.Abbreviation,
						output.Truncate(d.Name, 55),
						api.ParseODataDate(d.From),
						api.ParseODataDate(d.To),
					}
				}
				output.Table([]string{"ID", "Abbr", "Name", "From", "To"}, rows)
			} else {
				output.Section("Federal Departments")
				rows := make([][]string, len(depts))
				for i, d := range depts {
					rows[i] = []string{
						fmt.Sprintf("%d", d.ID),
						d.Abbreviation,
						output.Truncate(d.Name, 70),
					}
				}
				output.Table([]string{"ID", "Abbr", "Name"}, rows)
			}
			fmt.Println("  Source: ws-old.parlament.ch")
		} else {
			output.JSON(depts)
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

	// vote flags
	parlVoteCmd.Flags().BoolVar(&parlVoteDetail, "detail", false, "Show per-member and per-party vote breakdown")

	// session flags
	parlSessionCmd.Flags().BoolVar(&parlSessionCurrent, "current", false, "Show only the most recent session")

	// summary flags
	parlSummaryCmd.Flags().StringVar(&parlSummaryPeriod, "period", "today", `Time period: today, week, month, "last N days"`)
	parlSummaryCmd.Flags().StringVar(&parlSummaryFrom, "from", "", "Start date (YYYY-MM-DD)")
	parlSummaryCmd.Flags().StringVar(&parlSummaryTo, "to", "", "End date (YYYY-MM-DD)")

	// department flags
	parlDepartmentCmd.Flags().BoolVar(&parlDepartmentHistoric, "historic", false, "Include historic departments (with From/To dates)")

	// events flags
	parlEventsCmd.Flags().StringVar(&parlEventsCategory, "category", "", "Filter by category (Session, Event, 'Press conference advance notice', ...)")
	parlEventsCmd.Flags().BoolVar(&parlEventsSessions, "sessions", false, "Shortcut for --category=Session")
	parlEventsCmd.Flags().IntVar(&parlEventsLimit, "limit", 50, "Max events to return")
	parlEventsCmd.Flags().BoolVar(&parlEventsAll, "all", false, "Include past events (default: upcoming only)")

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
	parlCmd.AddCommand(parlDepartmentCmd)
	parlCmd.AddCommand(parlEventsCmd)

	rootCmd.AddCommand(parlCmd)
}
