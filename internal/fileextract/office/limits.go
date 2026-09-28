package office

const (
	defaultMaxInputBytes        int64  = 50 << 20
	defaultMaxEntries           int    = 4096
	defaultMaxUncompressedBytes uint64 = 150 << 20
	defaultMaxXMLBytes          uint64 = 16 << 20
	defaultMaxOutputBytes       int    = 2 << 20
	defaultMaxSheets            int    = 200
	defaultMaxRowsPerSheet      int    = 10000
	defaultMaxCellsPerSheet     int    = 50000
	defaultMaxSlides            int    = 500
	outputTruncationNotice             = "[content truncated: Office extraction output exceeded safety limit]"
)

type Limits struct {
	MaxInputBytes        int64
	MaxEntries           int
	MaxUncompressedBytes uint64
	MaxXMLBytes          uint64
	MaxOutputBytes       int
	MaxSheets            int
	MaxRowsPerSheet      int
	MaxCellsPerSheet     int
	MaxSlides            int
}

func DefaultLimits() Limits {
	return Limits{
		MaxInputBytes:        defaultMaxInputBytes,
		MaxEntries:           defaultMaxEntries,
		MaxUncompressedBytes: defaultMaxUncompressedBytes,
		MaxXMLBytes:          defaultMaxXMLBytes,
		MaxOutputBytes:       defaultMaxOutputBytes,
		MaxSheets:            defaultMaxSheets,
		MaxRowsPerSheet:      defaultMaxRowsPerSheet,
		MaxCellsPerSheet:     defaultMaxCellsPerSheet,
		MaxSlides:            defaultMaxSlides,
	}
}

func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxInputBytes <= 0 {
		l.MaxInputBytes = d.MaxInputBytes
	}
	if l.MaxEntries <= 0 {
		l.MaxEntries = d.MaxEntries
	}
	if l.MaxUncompressedBytes <= 0 {
		l.MaxUncompressedBytes = d.MaxUncompressedBytes
	}
	if l.MaxXMLBytes <= 0 {
		l.MaxXMLBytes = d.MaxXMLBytes
	}
	if l.MaxOutputBytes <= 0 {
		l.MaxOutputBytes = d.MaxOutputBytes
	}
	if l.MaxSheets <= 0 {
		l.MaxSheets = d.MaxSheets
	}
	if l.MaxRowsPerSheet <= 0 {
		l.MaxRowsPerSheet = d.MaxRowsPerSheet
	}
	if l.MaxCellsPerSheet <= 0 {
		l.MaxCellsPerSheet = d.MaxCellsPerSheet
	}
	if l.MaxSlides <= 0 {
		l.MaxSlides = d.MaxSlides
	}
	return l
}
