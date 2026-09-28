package office

import (
	"context"
	"encoding/xml"
	"io"
	"strings"
)

func extractPPTX(ctx context.Context, a *archive, source string) (string, error) {
	pres, err := a.readXML(ctx, FormatPPTX, source, "ppt/presentation.xml")
	if err != nil {
		return "", err
	}
	slideRels, err := parsePresentationSlides(pres)
	if err != nil {
		return "", xmlError(FormatPPTX, source, "ppt/presentation.xml", err)
	}
	if len(slideRels) > a.limits.MaxSlides {
		return "", officeError(ErrTooLarge, FormatPPTX, source, nil, "slide count exceeds limit")
	}

	relsXML, err := a.readXML(ctx, FormatPPTX, source, "ppt/_rels/presentation.xml.rels")
	if err != nil {
		return "", err
	}
	rels, err := parseRelationships(relsXML)
	if err != nil {
		return "", xmlError(FormatPPTX, source, "ppt/_rels/presentation.xml.rels", err)
	}

	slides := make([][]string, 0, len(slideRels))
	for _, rid := range slideRels {
		target := relTarget("ppt/presentation.xml", rels[rid])
		if target == "" {
			return "", missingEntry(FormatPPTX, source, "relationship "+rid)
		}
		data, err := a.readXML(ctx, FormatPPTX, source, target)
		if err != nil {
			return "", err
		}
		lines, err := collectTextByLocalName(data, "t")
		if err != nil {
			return "", xmlError(FormatPPTX, source, target, err)
		}
		slides = append(slides, lines)
	}
	return renderPPTX(source, slides), nil
}

func parsePresentationSlides(data []byte) ([]string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	var out []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "sldId" {
			continue
		}
		if id := relID(se); id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}
