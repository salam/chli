package api

// SPARQL query templates for Fedlex. Use fmt.Sprintf to fill placeholders.

// QuerySRByNumber looks up a consolidated law by its exact SR number.
// Uses skos:notation with the id-systematique datatype for precise matching.
// Placeholder order: %s = SR number (e.g. "101", "311.0"), %s = language URI (e.g. DEU).
const QuerySRByNumber = `PREFIX jolux: <http://data.legilux.public.lu/resource/ontology/jolux#>
PREFIX dcterms: <http://purl.org/dc/terms/>
PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
SELECT DISTINCT ?uri ?title ?dateDoc ?inForceURI ?inForceLabel WHERE {
  ?tax a skos:Concept ;
       skos:notation "%s"^^<https://fedlex.data.admin.ch/vocabulary/notation-type/id-systematique> .
  ?uri jolux:classifiedByTaxonomyEntry ?tax ;
       a jolux:ConsolidationAbstract ;
       jolux:dateDocument ?dateDoc ;
       jolux:inForceStatus ?inForceURI ;
       jolux:isRealizedBy ?expr .
  ?expr jolux:language <http://publications.europa.eu/resource/authority/language/%s> ;
        jolux:title ?title .
  OPTIONAL { ?inForceURI skos:prefLabel ?inForceLabel . FILTER(LANG(?inForceLabel) = "de") }
} ORDER BY DESC(?dateDoc)`

// QuerySearchTitle searches for consolidated laws by title substring.
// Placeholder order: %s = search term (case-insensitive via LCASE), %s = language URI.
const QuerySearchTitle = `PREFIX jolux: <http://data.legilux.public.lu/resource/ontology/jolux#>
PREFIX dcterms: <http://purl.org/dc/terms/>
SELECT DISTINCT ?uri ?identifier ?title ?dateDoc ?inForce WHERE {
  ?uri a jolux:ConsolidationAbstract ;
       dcterms:identifier ?identifier ;
       jolux:dateDocument ?dateDoc ;
       jolux:inForceStatus ?inForce ;
       jolux:isRealizedBy ?expr .
  ?expr jolux:language <http://publications.europa.eu/resource/authority/language/%s> ;
        jolux:title ?title .
  FILTER(CONTAINS(LCASE(?title), LCASE("%s")))
} LIMIT 50`

// QueryBBLByYear fetches Federal Gazette entries for a given year.
// Placeholder: %s = year (e.g. "2024").
const QueryBBLByYear = `PREFIX jolux: <http://data.legilux.public.lu/resource/ontology/jolux#>
PREFIX dcterms: <http://purl.org/dc/terms/>
SELECT DISTINCT ?uri ?title ?dateDoc WHERE {
  ?uri a jolux:Act ;
       dcterms:identifier ?identifier ;
       jolux:dateDocument ?dateDoc ;
       jolux:publicationDate ?pubDate ;
       jolux:isRealizedBy ?expr .
  ?expr jolux:language <http://publications.europa.eu/resource/authority/language/%s> ;
        jolux:title ?title .
  FILTER(STRSTARTS(STR(?dateDoc), "%s"))
} ORDER BY DESC(?dateDoc) LIMIT 50`

// QueryConsultations searches for consultations.
// Placeholder order: %s = language URI, %s = optional FILTER clause (or empty string).
const QueryConsultations = `PREFIX jolux: <http://data.legilux.public.lu/resource/ontology/jolux#>
PREFIX dcterms: <http://purl.org/dc/terms/>
SELECT DISTINCT ?uri ?title ?dateDoc ?status WHERE {
  ?uri a jolux:ConsultationProcess ;
       jolux:dateDocument ?dateDoc ;
       jolux:consultationStatus ?status ;
       jolux:isRealizedBy ?expr .
  ?expr jolux:language <http://publications.europa.eu/resource/authority/language/%s> ;
        jolux:title ?title .
  %s
} ORDER BY DESC(?dateDoc) LIMIT 50`

// QueryTreaties searches for international treaties.
// Placeholder order: %s = language URI, %s = optional FILTER clause (or empty string).
const QueryTreaties = `PREFIX jolux: <http://data.legilux.public.lu/resource/ontology/jolux#>
PREFIX dcterms: <http://purl.org/dc/terms/>
SELECT DISTINCT ?uri ?title ?dateDoc ?partner WHERE {
  ?uri a jolux:Treaty ;
       jolux:dateDocument ?dateDoc ;
       jolux:isRealizedBy ?expr .
  ?expr jolux:language <http://publications.europa.eu/resource/authority/language/%s> ;
        jolux:title ?title .
  OPTIONAL { ?uri jolux:treatyParty ?partner }
  %s
} ORDER BY DESC(?dateDoc) LIMIT 50`

// QuerySRVersions fetches all consolidated versions of a law by SR number.
// Placeholder order: %s = SR number, %s = language URI (e.g. DEU).
const QuerySRVersions = `PREFIX jolux: <http://data.legilux.public.lu/resource/ontology/jolux#>
PREFIX dcterms: <http://purl.org/dc/terms/>
PREFIX skos: <http://www.w3.org/2004/02/skos/core#>
SELECT DISTINCT ?version ?dateApplicability ?title WHERE {
  ?tax a skos:Concept ;
       skos:notation "%s"^^<https://fedlex.data.admin.ch/vocabulary/notation-type/id-systematique> .
  ?abstract jolux:classifiedByTaxonomyEntry ?tax ;
            a jolux:ConsolidationAbstract .
  ?version jolux:isConsolidationOf ?abstract ;
           a jolux:Consolidation ;
           jolux:dateApplicability ?dateApplicability ;
           jolux:isRealizedBy ?expr .
  ?expr jolux:language <http://publications.europa.eu/resource/authority/language/%s> ;
        jolux:title ?title .
} ORDER BY DESC(?dateApplicability)`
