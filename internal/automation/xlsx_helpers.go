package automation

import (
	"archive/zip"
	"io"
	"os"
	"strings"
)

func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	for _, file := range zr.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, os.ErrNotExist
}

func xlsxColumnIndex(ref string) int {
	letters := ""
	for _, r := range ref {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			letters += strings.ToUpper(string(r))
			continue
		}
		break
	}
	if letters == "" {
		return -1
	}
	idx := 0
	for _, r := range letters {
		idx = idx*26 + int(r-'A'+1)
	}
	return idx - 1
}
