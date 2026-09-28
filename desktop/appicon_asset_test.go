package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var windowsIconSizes = []int{16, 20, 24, 30, 32, 36, 40, 48, 60, 64, 72, 96, 128, 256}

func TestAppIconPNGUsesPearlCloudArtwork(t *testing.T) {
	f, err := os.Open("build/appicon.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	assertPearlCloudIcon(t, img, 1024)
}

func TestWindowsICOUsesPearlCloudArtwork(t *testing.T) {
	images := make([]image.Image, 0, len(windowsIconSizes))
	for _, size := range windowsIconSizes {
		t.Run(fmt.Sprintf("%dx%d", size, size), func(t *testing.T) {
			img := decodeICOImage(t, "build/windows/icon.ico", size)
			assertPearlCloudIcon(t, img, size)
			if size <= 48 {
				assertWindowsSmallIconVisible(t, img, size)
			}
			if size <= 64 {
				assertWindowsTaskbarArtworkScale(t, img, size)
			}
			images = append(images, img)
		})
	}
	assertWindowsIconScaleContinuity(t, images, windowsIconSizes)
}

func TestPlatformIconAssetsUsePearlCloudArtwork(t *testing.T) {
	for _, size := range []int{16, 24, 32, 48, 64, 128, 256, 512} {
		path := filepath.Join("build", "linux", "icons", "hicolor", fmt.Sprintf("%dx%d", size, size), "apps", "kwu-desktop.png")
		t.Run("linux-"+fmt.Sprint(size), func(t *testing.T) {
			assertPNGIcon(t, path, size)
		})
	}
	for _, size := range []int{16, 32, 48} {
		path := filepath.Join("frontend", "public", fmt.Sprintf("favicon-%d.png", size))
		t.Run("favicon-"+fmt.Sprint(size), func(t *testing.T) {
			assertPNGIcon(t, path, size)
		})
	}
}

func TestFrontendDeclaresDedicatedFavicons(t *testing.T) {
	data, err := os.ReadFile("frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, size := range []int{16, 32, 48, 256} {
		name := "favicon.png"
		if size != 256 {
			name = fmt.Sprintf("favicon-%d.png", size)
		}
		want := fmt.Sprintf(`sizes="%dx%d" href="/%s"`, size, size, name)
		if !strings.Contains(html, want) {
			t.Fatalf("frontend is missing dedicated %dpx favicon link", size)
		}
	}
}

func TestSVGIconsUseAccessibleBrandName(t *testing.T) {
	for _, path := range []string{
		"build/appicon.svg",
		"build/linux/icons/hicolor/scalable/apps/kwu-desktop.svg",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, `aria-label="云智企办"`) || !strings.Contains(text, "<title>云智企办</title>") {
			t.Fatalf("%s must identify the 云智企办 icon", path)
		}
		if strings.Contains(text, `aria-label="KWU"`) {
			t.Fatalf("%s still contains the old KWU label", path)
		}
	}
}

func TestMacICNSIncludesSmallAndModernFrames(t *testing.T) {
	data, err := os.ReadFile("build/darwin/icon.icns")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 8 || string(data[:4]) != "icns" || int(binary.BigEndian.Uint32(data[4:8])) != len(data) {
		t.Fatal("invalid ICNS header")
	}
	found := map[string]bool{}
	for offset := 8; offset < len(data); {
		if offset+8 > len(data) {
			t.Fatal("truncated ICNS element")
		}
		kind := string(data[offset : offset+4])
		length := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		if length < 8 || offset+length > len(data) {
			t.Fatalf("invalid ICNS %s element length %d", kind, length)
		}
		found[kind] = true
		offset += length
	}
	for _, kind := range []string{"icp4", "icp5", "icp6", "ic07", "ic08", "ic09", "ic10", "ic11", "ic12", "ic13", "ic14"} {
		if !found[kind] {
			t.Fatalf("ICNS is missing %s", kind)
		}
	}
}

func TestBrandWordmarkVariantsAreReadable(t *testing.T) {
	onLight := decodePNG(t, "frontend/src/assets/brand-wordmark.png")
	onDark := decodePNG(t, "frontend/src/assets/brand-wordmark-on-dark.png")
	for name, img := range map[string]image.Image{"on-light": onLight, "on-dark": onDark} {
		if got := img.Bounds().Size(); got.X != 422 || got.Y != 135 {
			t.Fatalf("%s wordmark size = %dx%d, want 422x135", name, got.X, got.Y)
		}
	}
	lightLuma := averageOpaqueLuma(onLight, 140)
	darkLuma := averageOpaqueLuma(onDark, 140)
	if lightLuma >= 80 {
		t.Fatalf("on-light wordmark text is too light: average=%d", lightLuma)
	}
	if darkLuma <= 220 {
		t.Fatalf("on-dark wordmark text is too dark: average=%d", darkLuma)
	}
}

func assertPNGIcon(t *testing.T, path string, size int) {
	t.Helper()
	assertPearlCloudIcon(t, decodePNG(t, path), size)
}

func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func averageOpaqueLuma(img image.Image, minX int) int {
	total, count := 0, 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := max(img.Bounds().Min.X, minX); x < img.Bounds().Max.X; x++ {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 == 0 {
				continue
			}
			total += (299*int(r16>>8) + 587*int(g16>>8) + 114*int(b16>>8)) / 1000
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func assertPearlCloudIcon(t *testing.T, img image.Image, size int) {
	t.Helper()

	bounds := img.Bounds()
	if bounds.Dx() != size || bounds.Dy() != size {
		t.Fatalf("app icon must be square, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	_, _, _, centerAlpha := img.At(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+bounds.Dy()/2).RGBA()
	if centerAlpha == 0 {
		t.Fatal("app icon center must contain visible artwork")
	}

	_, _, _, cornerAlpha := img.At(bounds.Min.X, bounds.Min.Y).RGBA()
	if cornerAlpha > 0x0505 {
		t.Fatal("app icon corners must remain transparent")
	}

	bluePixels := 0
	warmPixels := 0
	lightPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 == 0 {
				continue
			}
			r, g, b := int(r16>>8), int(g16>>8), int(b16>>8)
			if b > r+20 && b > g+10 {
				bluePixels++
			}
			if r > g+20 && r > b+20 {
				warmPixels++
			}
			if r > 170 && g > 165 && b > 155 {
				lightPixels++
			}
		}
	}
	if want := max(1, size*size/12); bluePixels < want {
		t.Fatalf("app icon must contain blue cloud artwork, blue pixels=%d want>=%d", bluePixels, want)
	}
	if want := max(1, size*size/20); warmPixels < want {
		t.Fatalf("app icon must contain warm cloud artwork, warm pixels=%d want>=%d", warmPixels, want)
	}
	if want := max(1, size*size/8); lightPixels < want {
		t.Fatalf("app icon must contain a pearl-white tile background, light pixels=%d want>=%d", lightPixels, want)
	}
}

func assertWindowsSmallIconVisible(t *testing.T, img image.Image, size int) {
	t.Helper()

	const taskbarChannel = 238
	minX, minY, maxX, maxY := size, size, -1, -1
	visiblePixels := 0
	definedEdgePixels := 0
	harshEdgePixels := 0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			a := int(a16 >> 8)
			r := int(r16>>8) + taskbarChannel*(255-a)/255
			g := int(g16>>8) + taskbarChannel*(255-a)/255
			b := int(b16>>8) + taskbarChannel*(255-a)/255
			contrast := max(abs(r-taskbarChannel), abs(g-taskbarChannel), abs(b-taskbarChannel))
			if x == 0 || x == size-1 || y == 0 || y == size-1 {
				if contrast >= 35 {
					definedEdgePixels++
				}
				if contrast >= 80 {
					harshEdgePixels++
				}
			}
			if contrast < 30 {
				continue
			}
			visiblePixels++
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
	}

	if maxX < minX || maxY < minY {
		t.Fatal("Windows small icon must remain visible on a light taskbar")
	}
	minSpan := size * 9 / 10
	if width, height := maxX-minX+1, maxY-minY+1; width < minSpan || height < minSpan {
		t.Fatalf("Windows small icon looks undersized on a light taskbar: visible=%dx%d want>=%dx%d", width, height, minSpan, minSpan)
	}
	if want := size * size * 2 / 5; visiblePixels < want {
		t.Fatalf("Windows small icon needs stronger visual weight: visible pixels=%d want>=%d", visiblePixels, want)
	}
	edgePixels := size*4 - 4
	if want := edgePixels * 2 / 5; definedEdgePixels < want {
		t.Fatalf("Windows small icon outer frame blends into a light taskbar: defined edge pixels=%d want>=%d", definedEdgePixels, want)
	}
	if limit := edgePixels / 10; harshEdgePixels > limit {
		t.Fatalf("Windows small icon outer frame is too dark: harsh edge pixels=%d want<=%d", harshEdgePixels, limit)
	}
}

func assertWindowsTaskbarArtworkScale(t *testing.T, img image.Image, size int) {
	t.Helper()

	minX, minY, maxX, maxY := size, size, -1, -1
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 < 0x8080 {
				continue
			}
			r, g, b := int(r16>>8), int(g16>>8), int(b16>>8)
			highest := max(r, max(g, b))
			lowest := min(r, min(g, b))
			if highest-lowest < 55 {
				continue
			}
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
	}

	if maxX < minX || maxY < minY {
		t.Fatal("Windows taskbar frame must retain the coloured cloud artwork")
	}
	minWidth := size * 9 / 10
	minHeight := size * 4 / 5
	if width, height := maxX-minX+1, maxY-minY+1; width < minWidth || height < minHeight {
		t.Fatalf(
			"Windows taskbar artwork is optically undersized: coloured=%dx%d want>=%dx%d",
			width,
			height,
			minWidth,
			minHeight,
		)
	}
}

func assertWindowsIconScaleContinuity(t *testing.T, images []image.Image, sizes []int) {
	t.Helper()
	if len(images) != len(sizes) {
		t.Fatalf("icon image count=%d want=%d", len(images), len(sizes))
	}
	previousCoverage := -1.0
	for i, img := range images {
		size := sizes[i]
		visible := 0
		minX, minY, maxX, maxY := size, size, -1, -1
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				_, _, _, alpha := img.At(x, y).RGBA()
				if alpha <= 0x0505 {
					continue
				}
				visible++
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
		if minX > maxX || minY > maxY {
			t.Fatalf("%dx%d frame is empty", size, size)
		}
		minSpan := size * 9 / 10
		if maxX-minX+1 < minSpan || maxY-minY+1 < minSpan {
			t.Fatalf("%dx%d frame is optically undersized: bbox=%dx%d", size, size, maxX-minX+1, maxY-minY+1)
		}
		coverage := float64(visible) / float64(size*size)
		if previousCoverage >= 0 && previousCoverage-coverage > 0.10 {
			t.Fatalf("icon visual weight jumps between %dpx and %dpx: %.1f%% -> %.1f%%", sizes[i-1], size, previousCoverage*100, coverage*100)
		}
		previousCoverage = coverage
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func decodeICOImage(t *testing.T, path string, size int) image.Image {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := bytes.NewReader(data)

	var header struct {
		Reserved uint16
		Type     uint16
		Count    uint16
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		t.Fatal(err)
	}
	if header.Reserved != 0 || header.Type != 1 {
		t.Fatalf("invalid ICO header: reserved=%d type=%d", header.Reserved, header.Type)
	}

	type iconEntry struct {
		Width       uint8
		Height      uint8
		ColorCount  uint8
		Reserved    uint8
		Planes      uint16
		BitCount    uint16
		BytesInRes  uint32
		ImageOffset uint32
	}

	entries := make([]iconEntry, header.Count)
	for i := range entries {
		if err := binary.Read(r, binary.LittleEndian, &entries[i]); err != nil {
			t.Fatal(err)
		}
	}

	expectedSizes := make(map[int]bool, len(windowsIconSizes))
	for _, expectedSize := range windowsIconSizes {
		expectedSizes[expectedSize] = false
	}
	targetIndex := -1
	for i, entry := range entries {
		width := int(entry.Width)
		height := int(entry.Height)
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		if width != height {
			t.Fatalf("ICO image must be square, got %dx%d", width, height)
		}
		if _, ok := expectedSizes[width]; ok {
			expectedSizes[width] = true
		}
		if width == size {
			targetIndex = i
		}
	}
	for expectedSize, found := range expectedSizes {
		if !found {
			t.Fatalf("ICO is missing %dx%d image", expectedSize, expectedSize)
		}
	}
	if targetIndex < 0 {
		t.Fatalf("ICO is missing %dx%d image", size, size)
	}

	entry := entries[targetIndex]
	end := int(entry.ImageOffset + entry.BytesInRes)
	if end > len(data) {
		t.Fatalf("ICO image offset exceeds file size: offset=%d size=%d file=%d", entry.ImageOffset, entry.BytesInRes, len(data))
	}
	img, err := png.Decode(bytes.NewReader(data[entry.ImageOffset:end]))
	if err != nil {
		t.Fatal(err)
	}
	return img
}
