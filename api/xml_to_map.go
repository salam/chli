package api

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// XMLToMap parses arbitrary XML into a generic structure suitable for JSON encoding.
// The root element's name is dropped; the returned value is the root's parsed body.
// Rules:
//   - An element with only character data becomes a string (trimmed).
//   - An element with child elements becomes a map[string]any keyed by child local name.
//   - Repeated child elements with the same name collapse into a []any.
//   - Mixed content stores the trimmed text under "#text".
//   - Attributes and namespaces are discarded.
func XMLToMap(data []byte) (any, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("XMLToMap: no root element")
		}
		if err != nil {
			return nil, fmt.Errorf("XMLToMap: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return parseXMLElement(dec, se)
		}
	}
}

func parseXMLElement(dec *xml.Decoder, start xml.StartElement) (any, error) {
	var textBuf strings.Builder
	children := map[string][]any{}
	order := []string{}

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			val, err := parseXMLElement(dec, t)
			if err != nil {
				return nil, err
			}
			name := t.Name.Local
			if _, seen := children[name]; !seen {
				order = append(order, name)
			}
			children[name] = append(children[name], val)
		case xml.EndElement:
			text := strings.TrimSpace(textBuf.String())
			if len(children) == 0 {
				return text, nil
			}
			out := map[string]any{}
			for _, k := range order {
				vals := children[k]
				if len(vals) == 1 {
					out[k] = vals[0]
				} else {
					out[k] = vals
				}
			}
			if text != "" {
				out["#text"] = text
			}
			return out, nil
		case xml.CharData:
			textBuf.Write(t)
		}
	}
}
