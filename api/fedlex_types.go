package api

// SPARQLValue represents a single value in a SPARQL result binding.
type SPARQLValue struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	XMLLang  string `json:"xml:lang,omitempty"`
	DataType string `json:"datatype,omitempty"`
}

// SPARQLResult is the standard SPARQL JSON response format.
type SPARQLResult struct {
	Head struct {
		Vars []string `json:"vars"`
	} `json:"head"`
	Results struct {
		Bindings []map[string]SPARQLValue `json:"bindings"`
	} `json:"results"`
}

// SREntry represents a Swiss law entry from the SR (Systematische Rechtssammlung).
type SREntry struct {
	URI          string `json:"uri"`
	Title        string `json:"title"`
	DateDoc      string `json:"dateDocument"`
	InForceURI   string `json:"inForceURI"`
	InForceLabel string `json:"inForceLabel"`
}

// FedlexSearchResult represents a search result from Fedlex.
type FedlexSearchResult struct {
	URI        string `json:"uri"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	DateDoc    string `json:"dateDocument"`
	InForce    string `json:"inForceStatus"`
}

// BBLEntry represents a Federal Gazette (Bundesblatt) entry.
type BBLEntry struct {
	URI     string `json:"uri"`
	Title   string `json:"title"`
	DateDoc string `json:"dateDocument"`
}

// ConsultationEntry represents a consultation entry.
type ConsultationEntry struct {
	URI     string `json:"uri"`
	Title   string `json:"title"`
	DateDoc string `json:"dateDocument"`
	Status  string `json:"status"`
}

// TreatyEntry represents an international treaty entry.
type TreatyEntry struct {
	URI     string `json:"uri"`
	Title   string `json:"title"`
	DateDoc string `json:"dateDocument"`
	Partner string `json:"partner"`
}
