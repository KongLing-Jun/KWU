package office

import (
	"encoding/xml"
	"io"
	"strings"
)

func parseRelationships(data []byte) (map[string]string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	out := map[string]string{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Relationship" {
			continue
		}
		id := attr(se, "Id")
		target := attr(se, "Target")
		if id != "" && target != "" {
			out[id] = target
		}
	}
	return out, nil
}
