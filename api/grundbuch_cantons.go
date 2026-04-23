package api

// Cantons is the per-canton capability registry for `chli grundbuch`. Each
// record was compiled from public canton sources during Phase 1 research
// (2026-04) and pre-loaded into the binary so the `canton` / `cantons`
// subcommands work offline.
//
// Fields marked with a Caveat are known to be time-sensitive (tariffs change
// annually on Jan 1) or were only partially documented in research; the CLI
// surfaces these caveats alongside the data instead of silently guessing.
//
// Tiering:
//   T1 — canton viewer exposes owner unauthenticated (BS, BL, SZ, TG; FR in
//        principle, but JSP frameset makes it viewer-only from CLI POV).
//   T2 — semi-public / rolling out (UR, GL).
//   T3 — free but gated: SMS to CH mobile or daily quota (ZH, ZG, AG, LU).
//   T4 — free but full eID required (BE — AGOV).
//   T5 — no public owner: Terravis/Intercapi/counter (rest).
//
// All parcels (geometry, EGRID, area) are reachable via the federal
// aggregated layer at api3.geo.admin.ch regardless of tier.

// intPtr is a readability helper for the CostSpec fields.
func intPtr(n int) *int { return &n }

const verifiedAt = "2026-04-21"

var Cantons = map[string]CantonCapability{
	"ZH": {
		Code: "ZH",
		Name: map[string]string{"de": "Zürich", "fr": "Zurich", "it": "Zurigo", "en": "Zurich"},
		Tier: TierT3,
		ParcelPortal: PortalRef{
			Name: "GIS-Browser Kanton Zürich",
			URL:  "https://maps.zh.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://maps.zh.ch/?topic=DLGOWfarbigZH",
			Type:  "viewer",
			Notes: "SMS code to Swiss mobile phone; 5 queries/day. Not automatable from a CLI.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Notariate Zürich (Grundbuchauszug)", URL: "https://www.notariate-zh.ch/de/grundbuch/", Type: "office"},
			{Name: "Elektronische Eigentumsabfrage", URL: "https://www.notariate-zh.ch/de/grundbuch/elektronische-eigentumsabfrage", Type: "viewer"},
		},
		AuthModel:       AuthSMSPhone,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(30), Notes: "Beglaubigter Auszug; online lookup itself is free."},
		GrundbuchamtURL: "https://www.notariate-zh.ch/de/grundbuch/",
		LegalNotes:      "ZGB Art. 970 — owner name is federally public but ZH gates online retrieval via SMS. Owners may block online disclosure (Sperrung) since Jan 2025.",
		Caveats:         []string{"Exact 2026 tariff for certified extract unverified."},
		VerifiedAt:      verifiedAt,
	},
	"BE": {
		Code: "BE",
		Name: map[string]string{"de": "Bern", "fr": "Berne", "it": "Berna", "en": "Bern"},
		Tier: TierT4,
		ParcelPortal: PortalRef{
			Name: "Geoportal Kanton Bern",
			URL:  "https://www.map.geo.be.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://grudis-public.apps.be.ch/grudis-public/",
			Type:  "viewer",
			Notes: "Free access with BE-Login (AGOV federal eID). Single-parcel lookup only; batch queries blocked.",
		},
		OwnerOrder: []PortalRef{
			{Name: "GRUDIS Public", URL: "https://grudis-public.apps.be.ch/grudis-public/", Type: "viewer"},
			{Name: "Grundbuchamt Bern — Bestellungen", URL: "https://www.gba.dij.be.ch/de/start/dienstleistungen/bestellungen.html", Type: "form"},
		},
		AuthModel:       AuthAGOV,
		Cost:            CostSpec{FixedCHF: intPtr(20), Notes: "CHF 10 per additional same-owner property. GRUDIS Public quick lookup is free."},
		GrundbuchamtURL: "https://www.gba.dij.be.ch/",
		LegalNotes:      "ZGB Art. 970 — BE implements public access via AGOV-authenticated GRUDIS Public portal.",
		VerifiedAt:      verifiedAt,
	},
	"LU": {
		Code: "LU",
		Name: map[string]string{"de": "Luzern", "fr": "Lucerne", "it": "Lucerna", "en": "Lucerne"},
		Tier: TierT3,
		ParcelPortal: PortalRef{
			Name: "GeoPortal Luzern",
			URL:  "https://geoportal.lu.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://www.grundbuch.lu.ch/",
			Type:  "viewer",
			Notes: "Up to 10 free owner queries per day per user.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Auszugsbestellung", URL: "https://grundbuch.lu.ch/onlinedienste/auszugsbestellung", Type: "form"},
			{Name: "GRAVIS (professional)", URL: "https://gravis.lu.ch", Type: "rest"},
		},
		AuthModel:       AuthSMSPhone,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(30), Notes: "Per Grundbuchgebührentarif; consumer lookup free within daily quota."},
		GrundbuchamtURL: "https://grundbuch.lu.ch/",
		LegalNotes:      "ZGB Art. 970. GRAVIS (professional) requires written application + contract.",
		Caveats:         []string{"Exact 2026 tariff unverified."},
		VerifiedAt:      verifiedAt,
	},
	"UR": {
		Code: "UR",
		Name: map[string]string{"de": "Uri", "fr": "Uri", "it": "Uri", "en": "Uri"},
		Tier: TierT2,
		ParcelPortal: PortalRef{
			Name: "geo.ur.ch",
			URL:  "https://geo.ur.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://oereb.ur.ch/",
			Type:  "viewer",
			Notes: "ÖREB viewer historically exposes owner name without login. CAPTCHA + rate limit after 2021 dnip.ch disclosure.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbucheinsicht Uri", URL: "https://www.ur.ch/dienstleistungen/3868", Type: "form"},
		},
		AuthModel:       AuthNone,
		Cost:            CostSpec{MinCHF: intPtr(30), MaxCHF: intPtr(50), Unpriced: true, Notes: "Tariff not published on canton page."},
		GrundbuchamtURL: "https://www.ur.ch/aemter/845",
		LegalNotes:      "ZGB Art. 970 explicitly affirmed — 'jede Person kann ohne Nachweis einer Berechtigung erfahren, wer Eigentümer eines Grundstücks ist'.",
		Caveats:         []string{"Current state of owner field in ÖREB viewer not re-verified in 2026 — may have been further restricted."},
		VerifiedAt:      verifiedAt,
	},
	"SZ": {
		Code: "SZ",
		Name: map[string]string{"de": "Schwyz", "fr": "Schwytz", "it": "Svitto", "en": "Schwyz"},
		Tier: TierT1,
		ParcelPortal: PortalRef{
			Name: "WebGIS Schwyz",
			URL:  "https://map.geo.sz.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://map.geo.sz.ch/",
			Type:  "viewer",
			Notes: "Owner + address visible in WebGIS property description; canton disclaims legal validity.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Terravis Auskunftsportal", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
			{Name: "Notariate Schwyz", URL: "https://www.sz.ch/", Type: "office"},
		},
		AuthModel:       AuthNone,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(50), Notes: "WebGIS read is free; certified extract via notary."},
		GrundbuchamtURL: "https://www.sz.ch/behoerden/verwaltung/umweltdepartement/amt-fuer-geoinformation.html",
		LegalNotes:      "SZ WebGIS is one of the strongest public owner signals in CH; owner info not legally binding.",
		VerifiedAt:      verifiedAt,
	},
	"OW": {
		Code: "OW",
		Name: map[string]string{"de": "Obwalden", "fr": "Obwald", "it": "Obvaldo", "en": "Obwalden"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Plan für das Grundbuch OW",
			URL:  "https://www.gis-daten.ch/map/plan_fuer_grundbuch_bund_ow",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuchamt Sarnen", URL: "https://www.ow.ch/fachbereiche/1834", Type: "office"},
			{Name: "Aussenstelle Engelberg", URL: "https://www.ow.ch/fachbereiche/1837", Type: "office"},
		},
		AuthModel:       AuthCounter,
		Cost:            CostSpec{Unpriced: true, Notes: "Tariff not published online."},
		GrundbuchamtURL: "https://www.ow.ch/fachbereiche/1834",
		LegalNotes:      "ZGB Art. 970 applies but OW has no online self-service for owner data.",
		Caveats:         []string{"Exact 2026 tariff unverified."},
		VerifiedAt:      verifiedAt,
	},
	"NW": {
		Code: "NW",
		Name: map[string]string{"de": "Nidwalden", "fr": "Nidwald", "it": "Nidvaldo", "en": "Nidwalden"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Plan für das Grundbuch NW",
			URL:  "https://www.gis-daten.ch/map/plan_fuer_grundbuch_bund_nw",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuch Nidwalden (online form)", URL: "https://www.nw.ch/grundbuchonline", Type: "form"},
			{Name: "Grundbuchdienste", URL: "https://www.nw.ch/grundbuchdienste/2326", Type: "office"},
		},
		AuthModel:       AuthCounter,
		Cost:            CostSpec{FixedCHF: intPtr(30), Notes: "Per Grundstück; online order requires 'Benutzerkonto für Online-Schalter'."},
		GrundbuchamtURL: "https://www.nw.ch/grundbuch/2491",
		LegalNotes:      "ZGB Art. 970. Terravis partially active — Dallenwil and Wolfenschiessen excluded.",
		VerifiedAt:      verifiedAt,
	},
	"GL": {
		Code: "GL",
		Name: map[string]string{"de": "Glarus", "fr": "Glaris", "it": "Glarona", "en": "Glarus"},
		Tier: TierT2,
		ParcelPortal: PortalRef{
			Name: "GeoViewer Glarus",
			URL:  "https://map.geo.gl.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://map.geo.gl.ch/",
			Type:  "viewer",
			Notes: "Grundstücksinformation tool rolling out; canton-wide owner lookup in progress.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuchamt Glarus", URL: "https://www.gl.ch/verwaltung/volkswirtschaft-und-inneres/wirtschaft-und-arbeit/grundbuchamt.html/1037", Type: "office"},
			{Name: "my.gl.ch Serviceportal", URL: "https://my.gl.ch/", Type: "form"},
		},
		AuthModel:       AuthNone,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(50), Unpriced: true},
		GrundbuchamtURL: "https://www.gl.ch/verwaltung/volkswirtschaft-und-inneres/wirtschaft-und-arbeit/grundbuchamt.html/1037",
		LegalNotes:      "ZGB Art. 970 affirmed — 'jede Person kann ohne Nachweis einer Berechtigung erfahren, wer Eigentümer eines Grundstücks ist'.",
		Caveats:         []string{"Owner lookup in the viewer is currently limited to Glarus municipality; canton-wide rollout underway.", "Exact 2026 tariff unverified."},
		VerifiedAt:      verifiedAt,
	},
	"ZG": {
		Code: "ZG",
		Name: map[string]string{"de": "Zug", "fr": "Zoug", "it": "Zugo", "en": "Zug"},
		Tier: TierT3,
		ParcelPortal: PortalRef{
			Name: "ZugMap",
			URL:  "https://zugmap.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://zugmap.ch/",
			Type:  "viewer",
			Notes: "Owner query requires Swiss mobile phone (SMS). Not automatable.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Amt für Grundbuch und Geoinformation", URL: "https://zg.ch/de/planen-bauen/grundbuch", Type: "office"},
		},
		AuthModel:       AuthSMSPhone,
		Cost:            CostSpec{MinCHF: intPtr(45), Notes: "Minimum per § 13 Grundbuchgebührentarif — ZG is time-based."},
		GrundbuchamtURL: "https://zg.ch/de/planen-bauen/grundbuch",
		LegalNotes:      "ZGB Art. 970; ZG integrates Grundbuch and Geoinformation in one office.",
		VerifiedAt:      verifiedAt,
	},
	"FR": {
		Code: "FR",
		Name: map[string]string{"de": "Freiburg", "fr": "Fribourg", "it": "Friburgo", "en": "Fribourg"},
		Tier: TierT1,
		ParcelPortal: PortalRef{
			Name: "Portail cartographique Fribourg",
			URL:  "https://maps.fr.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://keycloak.fr.ch/rfpublic/",
			Type:  "viewer",
			Notes: "RFpublic: free, login-free owner lookup. Note: JSP frameset app — no JSON API; not automatable from a CLI.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Intercapi Fribourg", URL: "https://www.rf.fr.ch/intercapi", Type: "rest"},
			{Name: "Registre Foncier FR", URL: "https://www.fr.ch/territoire-amenagement-et-constructions/registre-foncier", Type: "office"},
		},
		AuthModel:       AuthNone,
		Cost:            CostSpec{Unpriced: true, Notes: "RFpublic consultation is free; Intercapi certified extract per cantonal tariff."},
		GrundbuchamtURL: "https://www.fr.ch/territoire-amenagement-et-constructions/registre-foncier",
		LegalNotes:      "ZGB Art. 970. Only canton in CH with an unambiguously public, unauthenticated owner-lookup web app — but served as JSP frameset, so the CLI surfaces the URL rather than scraping it.",
		VerifiedAt:      verifiedAt,
	},
	"SO": {
		Code: "SO",
		Name: map[string]string{"de": "Solothurn", "fr": "Soleure", "it": "Soletta", "en": "Solothurn"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Geoportal Solothurn",
			URL:  "https://so.ch/verwaltung/bau-und-justizdepartement/amt-fuer-geoinformation/geoportal/",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuchauszug bestellen", URL: "https://so.ch/services/grundbuchauszug-bestellen/", Type: "form"},
			{Name: "Intercapi Solothurn (professional)", URL: "https://intercapi.so.ch", Type: "rest"},
		},
		AuthModel:       AuthProfessional,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(30), Unpriced: true, Notes: "Beglaubigt: +CHF 20 approximate. Intercapi for authorities/institutions only."},
		GrundbuchamtURL: "https://so.ch/verwaltung/finanzdepartement/grundbuchaemter/",
		LegalNotes:      "ZGB Art. 970. SO has no public owner-by-parcel endpoint; Intercapi is contract-only.",
		Caveats:         []string{"Exact 2026 tariff unverified."},
		VerifiedAt:      verifiedAt,
	},
	"BS": {
		Code: "BS",
		Name: map[string]string{"de": "Basel-Stadt", "fr": "Bâle-Ville", "it": "Basilea Città", "en": "Basel-Stadt"},
		Tier: TierT1,
		ParcelPortal: PortalRef{
			Name: "MapBS",
			URL:  "https://map.geo.bs.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://map.geo.bs.ch/eigentumsauskunft",
			Type:  "viewer",
			Notes: "Owner name(s) visible in public Eigentumsauskunft viewer; subject to owner opt-out and anti-scraping throttling.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuchauszug bestellen", URL: "https://www.bs.ch/bvd/grundbuch-und-vermessungsamt/grundbuch/grundbuchbeleg", Type: "form"},
		},
		AuthModel:       AuthNone,
		Cost:            CostSpec{Unpriced: true, Notes: "Tariff at gva.bs.ch/grundbuch/dienstleistungen/grundbuchgebuehren.html. Terravis currently suspended for BS."},
		GrundbuchamtURL: "https://www.gva.bs.ch/",
		LegalNotes:      "ZGB Art. 970 — BS is unusually open; owner names publicly visible in cantonal viewer.",
		Caveats:         []string{"Exact 2026 tariff unverified.", "Automated/bulk scraping blocked by canton."},
		VerifiedAt:      verifiedAt,
	},
	"BL": {
		Code: "BL",
		Name: map[string]string{"de": "Basel-Landschaft", "fr": "Bâle-Campagne", "it": "Basilea Campagna", "en": "Basel-Landschaft"},
		Tier: TierT1,
		ParcelPortal: PortalRef{
			Name: "GeoView BL",
			URL:  "https://geoview.bl.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://eigentumsauskunft.geo.bl.ch/",
			Type:  "viewer",
			Notes: "Owner name + address by EGRID or address search. Rate-limited against batch queries; owners may opt out.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuchauszug bestellen", URL: "https://forms.bl.ch/form/FMS-BL/SID_Zivilrechtsverwaltung_Grundbuchauszug/de", Type: "form"},
		},
		AuthModel:       AuthNone,
		Cost:            CostSpec{FixedCHF: intPtr(40), Notes: "CHF 10 per subjectively-linked parcel; postage CHF 1.20 (CH) / CHF 3 (abroad)."},
		GrundbuchamtURL: "https://www.baselland.ch/politik-und-behorden/direktionen/sicherheitsdirektion/zivilrechtsverwaltung/grundbuchamt",
		LegalNotes:      "ZGB Art. 970 'Interessennachweis' effectively fulfilled by the public portal. TerraVis available to notaries since Aug 2025.",
		VerifiedAt:      verifiedAt,
	},
	"SH": {
		Code: "SH",
		Name: map[string]string{"de": "Schaffhausen", "fr": "Schaffhouse", "it": "Sciaffusa", "en": "Schaffhausen"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "map.geo.sh.ch",
			URL:  "https://map.geo.sh.ch/",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuchamt Schaffhausen (email)", URL: "mailto:gbamt@ktsh.ch", Type: "email"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthProfessional,
		Cost:            CostSpec{Unpriced: true, Notes: "Per cantonal Gebührenordnung; not published centrally."},
		GrundbuchamtURL: "https://sh.ch/CMS/Webseite/Kanton-Schaffhausen/Beh-rde/Verwaltung/Volkswirtschaftsdepartement/Grundbuchamt---Notariat-1633241-DE.html",
		LegalNotes:      "ZGB Art. 970 — Interessennachweis required. Same-day email delivery for orders placed before 11:00.",
		Caveats:         []string{"Exact 2026 tariff unverified."},
		VerifiedAt:      verifiedAt,
	},
	"AR": {
		Code: "AR",
		Name: map[string]string{"de": "Appenzell Ausserrhoden", "fr": "Appenzell Rhodes-Extérieures", "it": "Appenzello Esterno", "en": "Appenzell Outer Rhodes"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Geoportal Appenzell Ausserrhoden",
			URL:  "https://www.geoportal.ch/ktar",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuch- und Beurkundungsinspektorat AR", URL: "https://ar.ch/verwaltung/departement-inneres-und-sicherheit/departementssekretariat/grundbuch-und-beurkundungsinspektorat/", Type: "office"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthProfessional,
		Cost:            CostSpec{Unpriced: true, Notes: "Per municipal/cantonal Gebührenordnung."},
		GrundbuchamtURL: "https://ar.ch/verwaltung/departement-inneres-und-sicherheit/departementssekretariat/grundbuch-und-beurkundungsinspektorat/",
		LegalNotes:      "ZGB Art. 970 — Interessennachweis required. The 'elektronisches Auskunftsportal' announced by AR is TerraVis (professional), not a public viewer.",
		Caveats:         []string{"Despite being a small canton, AR is not more open than average."},
		VerifiedAt:      verifiedAt,
	},
	"AI": {
		Code: "AI",
		Name: map[string]string{"de": "Appenzell Innerrhoden", "fr": "Appenzell Rhodes-Intérieures", "it": "Appenzello Interno", "en": "Appenzell Inner Rhodes"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Geoportal Appenzell Innerrhoden",
			URL:  "https://www.geoportal.ch/ktai",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuch und Erbschaftsamt AI", URL: "https://www.ai.ch/themen/planen-und-bauen/grundbuch-notariat/grundbuchauszug", Type: "form"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthCounter,
		Cost:            CostSpec{FixedCHF: intPtr(40), Notes: "Post delivery within ~1 week."},
		GrundbuchamtURL: "https://www.ai.ch/verwaltung/volkwirtschaftsdepartement/grundbuch-und-erbschaftsamt",
		LegalNotes:      "Despite Landsgemeinde tradition, AI is stricter than ZGB 970 default — order page explicitly demands proof of interest or power of attorney.",
		VerifiedAt:      verifiedAt,
	},
	"SG": {
		Code: "SG",
		Name: map[string]string{"de": "St. Gallen", "fr": "Saint-Gall", "it": "San Gallo", "en": "St. Gallen"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Geoportal St. Gallen",
			URL:  "https://www.geoportal.ch/ktsg",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuchämter SG (49 offices)", URL: "https://www.sg.ch/politik-verwaltung/gemeinden/grundbuch/grundbuchaemter.html", Type: "office"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthProfessional,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(50), Unpriced: true, Notes: "Per Gebührenordnung of each Grundbuchkreis."},
		GrundbuchamtURL: "https://www.sg.ch/politik-verwaltung/gemeinden/grundbuch/grundbuchaemter.html",
		LegalNotes:      "ZGB Art. 970. SG is highly decentralized (49 municipal/district Grundbuchämter, each with own order channel).",
		Caveats:         []string{"Exact 2026 tariff varies per Grundbuchkreis and is unverified."},
		VerifiedAt:      verifiedAt,
	},
	"GR": {
		Code: "GR",
		Name: map[string]string{"de": "Graubünden", "fr": "Grisons", "it": "Grigioni", "en": "Grisons"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "map.geo.gr.ch",
			URL:  "https://map.geo.gr.ch/",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Grundbuchinspektorat GR", URL: "https://www.gr.ch/DE/institutionen/verwaltung/dvs/giha/grundbuch/Seiten/grundbuchinspektorat.aspx", Type: "office"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthProfessional,
		Cost:            CostSpec{MinCHF: intPtr(50), Unpriced: true, Notes: "Art. 18 Gebührenverordnung — 'more than CHF 50' reported."},
		GrundbuchamtURL: "https://www.gr.ch/DE/institutionen/verwaltung/dvs/giha/grundbuch/Seiten/grundbuchinspektorat.aspx",
		LegalNotes:      "ZGB Art. 970 — Interessennachweis required. GR has 17 Grundbuchkreise; cantonal inspectorate supervises but does not issue extracts.",
		Caveats:         []string{"Exact 2026 tariff unverified."},
		VerifiedAt:      verifiedAt,
	},
	"AG": {
		Code: "AG",
		Name: map[string]string{"de": "Aargau", "fr": "Argovie", "it": "Argovia", "en": "Aargau"},
		Tier: TierT3,
		ParcelPortal: PortalRef{
			Name: "AGIS",
			URL:  "https://wms.geo.ag.ch/",
			Type: "wms",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://www.ag.ch/de/themen/planen-bauen/grundbuch-vermessung/grundbuch/das-grundbuch",
			Type:  "viewer",
			Notes: "Up to 10 free owner queries/day via AG Geoportal Grundeigentümerabfrage.",
		},
		OwnerOrder: []PortalRef{
			{Name: "Smart-Service-Portal (Grundbuchauszug)", URL: "https://www.ag.ch/de/smartserviceportal/dienstleistungen?dl=grundbuchauszug-f0b264bc-5549-417a-b03c-27b840dd5a42_de", Type: "form"},
		},
		AuthModel:       AuthSMSPhone,
		Cost:            CostSpec{FixedCHF: intPtr(30), Notes: "CHF 15 per additional same-owner property + CHF 1.20 postage."},
		GrundbuchamtURL: "https://www.ag.ch/de/themen/planen-bauen/grundbuch-vermessung/grundbuch/das-grundbuch",
		LegalNotes:      "ZGB Art. 970. AG also uses Intercapi for professional access.",
		VerifiedAt:      verifiedAt,
	},
	"TG": {
		Code: "TG",
		Name: map[string]string{"de": "Thurgau", "fr": "Thurgovie", "it": "Turgovia", "en": "Thurgau"},
		Tier: TierT1,
		ParcelPortal: PortalRef{
			Name: "ThurGIS",
			URL:  "https://map.geo.tg.ch/",
			Type: "viewer",
		},
		OwnerPublic: &OwnerEndpoint{
			URL:   "https://map.geo.tg.ch/",
			Type:  "viewer",
			Notes: "ThurGIS viewer publicly exposes owner (Eigentümerabfrage); opt-out possible.",
		},
		OwnerOrder: []PortalRef{
			{Name: "GNI Digitaler Schalter", URL: "https://gni.tg.ch/dienstleistungen/bestellung-grundbuchauszuege.html/12319", Type: "form"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthNone,
		Cost:            CostSpec{MinCHF: intPtr(50), Notes: "TG is at the higher end of CH cantons."},
		GrundbuchamtURL: "https://gni.tg.ch/",
		LegalNotes:      "TG and BL are the most openly queryable cantons for owner via GIS.",
		VerifiedAt:      verifiedAt,
	},
	"TI": {
		Code: "TI",
		Name: map[string]string{"de": "Tessin", "fr": "Tessin", "it": "Ticino", "en": "Ticino"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Geoportale Ticino",
			URL:  "https://map.geo.ti.ch/",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Ufficio del registro fondiario", URL: "https://www4.ti.ch/di/dg/rf/registro-fondiario/il-registro-fondiario/", Type: "office"},
		},
		AuthModel:       AuthCounter,
		Cost:            CostSpec{FixedCHF: intPtr(20), Notes: "+ ~CHF 20 for certification supplement; paper delivery by post only."},
		GrundbuchamtURL: "https://www4.ti.ch/di/dg/rf/registro-fondiario/il-registro-fondiario/",
		LegalNotes:      "TI intentionally keeps RF consultation offline for public (art. 30 LTORF). Written request required — no telephone orders.",
		VerifiedAt:      verifiedAt,
	},
	"VD": {
		Code: "VD",
		Name: map[string]string{"de": "Waadt", "fr": "Vaud", "it": "Vaud", "en": "Vaud"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Guichet cartographique VD / ASIT-VD",
			URL:  "https://map.geo.vd.ch/",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Intercapi (professional)", URL: "https://www.vd.ch/territoire-et-construction/registre-foncier", Type: "rest"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
			{Name: "Guichets Registre Foncier", URL: "https://www.vd.ch/territoire-et-construction/registre-foncier", Type: "office"},
		},
		AuthModel:       AuthProfessional,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(30), Notes: "Certified extract per Règlement RF-VD."},
		GrundbuchamtURL: "https://www.vd.ch/territoire-et-construction/registre-foncier",
		LegalNotes:      "ZGB Art. 970 — propriétaire never public in VD. Public individuals must request at a guichet RF with justified interest.",
		VerifiedAt:      verifiedAt,
	},
	"VS": {
		Code: "VS",
		Name: map[string]string{"de": "Wallis", "fr": "Valais", "it": "Vallese", "en": "Valais"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "Géoportail Valais",
			URL:  "https://map.geo.vs.ch/",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Consultation en ligne du RF-VS", URL: "https://www.vs.ch/web/srf/consultation-en-ligne-du-registre-foncier", Type: "rest"},
			{Name: "Intercapi Valais", URL: "https://www.vs.ch/de/web/srf/online-einsicht-in-die-grundbuchdaten", Type: "rest"},
		},
		AuthModel:       AuthSwissID,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(40), Notes: "Public consultation via 'portail des autorités valaisannes' account (free); certified extract ~CHF 20-30 + CHF 20 certification."},
		GrundbuchamtURL: "https://www.vs.ch/web/srf",
		LegalNotes:      "VS uniquely exposes public consultation of owner data for digitized communes — but requires account registration with the cantonal portal.",
		Caveats:         []string{"Coverage varies commune by commune.", "Exact 2026 tariff unverified."},
		VerifiedAt:      verifiedAt,
	},
	"NE": {
		Code: "NE",
		Name: map[string]string{"de": "Neuenburg", "fr": "Neuchâtel", "it": "Neuchâtel", "en": "Neuchâtel"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "SITN",
			URL:  "https://sitn.ne.ch/",
			Type: "viewer",
		},
		OwnerOrder: []PortalRef{
			{Name: "Service du géomatique et du registre foncier (SGRF)", URL: "https://www.ne.ch/autorites/DDTE/SGRF/Pages/accueil.aspx", Type: "office"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthProfessional,
		Cost:            CostSpec{MinCHF: intPtr(20), MaxCHF: intPtr(30), Unpriced: true, Notes: "NE tariff per cantonal regulation."},
		GrundbuchamtURL: "https://www.ne.ch/autorites/DDTE/SGRF/Pages/accueil.aspx",
		LegalNotes:      "ZGB Art. 970 — owner data not exposed via public API. SITN is well-documented for parcel geometry.",
		Caveats:         []string{"Exact 2026 tariff unverified."},
		VerifiedAt:      verifiedAt,
	},
	"GE": {
		Code: "GE",
		Name: map[string]string{"de": "Genf", "fr": "Genève", "it": "Ginevra", "en": "Geneva"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "SITG",
			URL:  "https://sitg.ge.ch/",
			Type: "rest",
		},
		OwnerOrder: []PortalRef{
			{Name: "e-démarches (certified extract)", URL: "https://www.ge.ch/consulter-registre-foncier", Type: "form"},
			{Name: "Intercapi (professional)", URL: "https://www.ge.ch/consulter-registre-foncier", Type: "rest"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthSwissID,
		Cost:            CostSpec{MinCHF: intPtr(15), MaxCHF: intPtr(50), Notes: "CHF 15-30 simple extract; CHF 30-50 certified. Per REmORFDIT rsGE E 1 50.06."},
		GrundbuchamtURL: "https://www.ge.ch/consulter-registre-foncier",
		LegalNotes:      "SITG is famously open for GIS data (WMS/WFS/REST) but deliberately excludes owner from open data. Certified extracts via e-démarches (SwissID).",
		VerifiedAt:      verifiedAt,
	},
	"JU": {
		Code: "JU",
		Name: map[string]string{"de": "Jura", "fr": "Jura", "it": "Giura", "en": "Jura"},
		Tier: TierT5,
		ParcelPortal: PortalRef{
			Name: "SIT-Jura",
			URL:  "https://geo.jura.ch/",
			Type: "wms",
		},
		OwnerOrder: []PortalRef{
			{Name: "Service du registre foncier et du registre du commerce (RFC)", URL: "https://www.jura.ch/fr/Autorites/Administration/DSJP/RFC/Service-du-registre-foncier-et-du-registre-du-commerce-RFC.html", Type: "office"},
			{Name: "Terravis (professional)", URL: "https://www.six-group.com/en/site/terravis.html", Type: "rest"},
		},
		AuthModel:       AuthCounter,
		Cost:            CostSpec{FixedCHF: intPtr(21), Notes: "CHF 10.50 per additional property + CHF 3 fees (+ CHF 1 per email). Payment by invoice joined to delivery."},
		GrundbuchamtURL: "https://www.jura.ch/fr/Autorites/Administration/DSJP/RFC/Service-du-registre-foncier-et-du-registre-du-commerce-RFC.html",
		LegalNotes:      "JU has a single RF office in Delémont. WMS public; WFS limited. Fair-use <10 req/min.",
		VerifiedAt:      verifiedAt,
	},
}
