package fonts

import (
	"context"
	"encoding/binary"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/encoding/charmap"
)

const (
	maxFontFiles = 4000

	maxFontFileSize = 256 << 20

	monospaceProbeA = 'i'
	monospaceProbeB = 'W'
	monospacePPEM   = 16
)

var nerdFontProbeRunes = []rune{0xF01A8, 0xE738, 0xF115, 0xE5FF}

const nerdFontProbeThreshold = 2

func scanLocalFonts(ctx context.Context) ([]Family, error) {
	directories := existingFontDirectories()
	seen := make(map[string]Family)
	var files int

	for _, directory := range directories {
		if files >= maxFontFiles {
			break
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		walkErr := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if files >= maxFontFiles {
				return fs.SkipAll
			}
			if err != nil || entry.IsDir() {

				return nil
			}
			if !isFontFile(path) {
				return nil
			}
			files++
			for _, family := range readFontFamilies(path) {
				key := strings.ToLower(family.Family)
				existing, ok := seen[key]
				if !ok {
					seen[key] = family
					continue
				}

				merged := existing
				if family.HasNerdIcons {
					merged.HasNerdIcons = true
				}
				if family.Monospace {
					merged.Monospace = true
				}

				merged.LocalizedNames = mergeLocalizedNames(merged.LocalizedNames, family.LocalizedNames)
				seen[key] = merged
			}
			return nil
		})
		if walkErr != nil {
			return nil, scanError(walkErr)
		}
	}

	families := make([]Family, 0, len(seen))
	for _, family := range seen {
		families = append(families, family)
	}
	sortFamilies(families)
	return families, nil
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	default:
		return false
	}
}

func readFontFamilies(path string) []Family {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil || info.Size() == 0 || info.Size() > maxFontFileSize {
		return nil
	}

	if strings.EqualFold(filepath.Ext(path), ".ttc") || strings.EqualFold(filepath.Ext(path), ".otc") {
		collection, err := sfnt.ParseCollectionReaderAt(file)
		if err != nil {
			return nil
		}

		offsets := ttcFontOffsets(file, info.Size())
		families := make([]Family, 0, collection.NumFonts())
		for index := 0; index < collection.NumFonts(); index++ {
			parsed, err := collection.Font(index)
			if err != nil {
				continue
			}
			family, ok := describeFont(parsed)
			if !ok {
				continue
			}
			if index < len(offsets) {
				family.LocalizedNames = readLocalizedNames(file, info.Size(), offsets[index], "")
			}
			preferEnglishFamilyName(&family)
			if !cssUsableFamilyName(family.Family) {
				continue
			}
			families = append(families, family)
		}
		return families
	}

	parsed, err := sfnt.ParseReaderAt(file)
	if err != nil {
		return nil
	}
	family, ok := describeFont(parsed)
	if !ok {
		return nil
	}

	family.LocalizedNames = readLocalizedNames(file, info.Size(), 0, "")
	preferEnglishFamilyName(&family)
	if !cssUsableFamilyName(family.Family) {
		return nil
	}
	return []Family{family}
}

func cssUsableFamilyName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.HasPrefix(trimmed, ".") {
		return false
	}
	return !strings.EqualFold(trimmed, "System Font")
}

func preferEnglishFamilyName(family *Family) {
	if english := strings.TrimSpace(family.LocalizedNames["en"]); english != "" && english != family.Family {
		family.Family = english
	}
	if len(family.LocalizedNames) == 0 {
		return
	}

	names := make(map[string]string, len(family.LocalizedNames))
	for key, value := range family.LocalizedNames {
		if strings.TrimSpace(value) == "" || value == family.Family {
			continue
		}
		names[key] = value
	}
	if len(names) == 0 {
		family.LocalizedNames = nil
		return
	}
	family.LocalizedNames = names
}

func describeFont(parsed *sfnt.Font) (Family, bool) {
	if parsed == nil || parsed.NumGlyphs() < 2 {
		return Family{}, false
	}

	name, err := parsed.Name(nil, sfnt.NameIDTypographicFamily)
	if err != nil || strings.TrimSpace(name) == "" {
		name, err = parsed.Name(nil, sfnt.NameIDFamily)
	}
	name = strings.TrimSpace(name)
	if err != nil || name == "" {
		return Family{}, false
	}
	return Family{
		Family:       name,
		Monospace:    isMonospace(parsed),
		HasNerdIcons: hasNerdFontIcons(parsed),
	}, true
}

const (
	nameTableTag = "name"

	nameIDFamily            = 1
	nameIDTypographicFamily = 16

	maxNameStringBytes = 1024

	maxSFNTTables       = 4096
	maxNameTableRecords = 4096
)

var windowsLocalizedNameLanguages = map[uint16]string{
	0x0409: "en",
	0x0809: "en",
	0x0804: "zh-Hans",
	0x0404: "zh-Hant",
	0x0C04: "zh-Hant",
	0x1404: "zh-Hant",
	0x0411: "ja",
	0x0412: "ko",
	0x0419: "ru",

	0x040C: "fr",
	0x080C: "fr",
	0x0C0C: "fr",
	0x100C: "fr",
}

var macintoshLocalizedNameLanguages = map[uint16]string{
	0:  "en",
	33: "zh-Hans",
	19: "zh-Hant",
	11: "ja",
	23: "ko",
	32: "ru",
	1:  "fr",
}

func readLocalizedNames(reader io.ReaderAt, size, tableDirectoryOffset int64, canonical string) map[string]string {
	tableOffset, tableLength, ok := findNameTable(reader, size, tableDirectoryOffset)
	if !ok {
		return nil
	}
	return readNameTableNames(reader, size, tableOffset, tableLength, canonical)
}

func findNameTable(reader io.ReaderAt, size, tableDirectoryOffset int64) (int64, int64, bool) {
	header := make([]byte, 12)
	if !readFullAt(reader, size, header, tableDirectoryOffset) {
		return 0, 0, false
	}
	numTables := int64(binary.BigEndian.Uint16(header[4:6]))
	if numTables <= 0 || numTables > maxSFNTTables {
		return 0, 0, false
	}
	recordsOffset := tableDirectoryOffset + 12
	recordsLength := numTables * 16
	if !validFontRange(recordsOffset, recordsLength, size) {
		return 0, 0, false
	}
	records := make([]byte, recordsLength)
	if !readFullAt(reader, size, records, recordsOffset) {
		return 0, 0, false
	}
	for index := int64(0); index < numTables; index++ {
		record := records[index*16 : (index+1)*16]
		if !isNameTableTag(record) {
			continue
		}
		offset := int64(binary.BigEndian.Uint32(record[8:12]))
		length := int64(binary.BigEndian.Uint32(record[12:16]))
		if length < 6 || !validFontRange(offset, length, size) {
			return 0, 0, false
		}
		return offset, length, true
	}
	return 0, 0, false
}

func isNameTableTag(record []byte) bool {
	return record[0] == nameTableTag[0] && record[1] == nameTableTag[1] &&
		record[2] == nameTableTag[2] && record[3] == nameTableTag[3]
}

type localizedNameCandidate struct {
	value string
	score int
}

func readNameTableNames(reader io.ReaderAt, size, tableOffset, tableLength int64, canonical string) map[string]string {
	header := make([]byte, 6)
	if !readFullAt(reader, size, header, tableOffset) {
		return nil
	}
	count := int64(binary.BigEndian.Uint16(header[2:4]))
	stringOffset := int64(binary.BigEndian.Uint16(header[4:6]))
	if count <= 0 || count > maxNameTableRecords {
		return nil
	}

	if stringOffset < 6+12*count || stringOffset > tableLength {
		return nil
	}
	tableEnd := tableOffset + tableLength
	recordsLength := count * 12
	if !validFontRange(tableOffset+6, recordsLength, tableEnd) {
		return nil
	}
	records := make([]byte, recordsLength)
	if !readFullAt(reader, size, records, tableOffset+6) {
		return nil
	}

	candidates := make(map[string]localizedNameCandidate, 4)
	for index := int64(0); index < count; index++ {
		record := records[index*12 : (index+1)*12]
		platformID := binary.BigEndian.Uint16(record[0:2])
		encodingID := binary.BigEndian.Uint16(record[2:4])
		languageID := binary.BigEndian.Uint16(record[4:6])
		nameID := binary.BigEndian.Uint16(record[6:8])
		length := int64(binary.BigEndian.Uint16(record[8:10]))
		offset := int64(binary.BigEndian.Uint16(record[10:12]))

		if length == 0 || length > maxNameStringBytes {
			continue
		}
		key, score, ok := localizedRecordKey(platformID, encodingID, languageID, nameID)
		if !ok {
			continue
		}
		start := tableOffset + stringOffset + offset
		if !validFontRange(start, length, tableEnd) {
			continue
		}
		raw := make([]byte, length)
		if !readFullAt(reader, size, raw, start) {
			continue
		}
		value, ok := decodeNameString(platformID, raw)
		if !ok {
			continue
		}
		value = normalizeNameValue(value)
		if value == "" || value == canonical {
			continue
		}
		if current, exists := candidates[key]; exists && current.score >= score {
			continue
		}
		candidates[key] = localizedNameCandidate{value: value, score: score}
	}

	if len(candidates) == 0 {
		return nil
	}
	names := make(map[string]string, len(candidates))
	for key, candidate := range candidates {
		names[key] = candidate.value
	}
	return names
}

func localizedRecordKey(platformID, encodingID, languageID, nameID uint16) (string, int, bool) {
	if nameID != nameIDFamily && nameID != nameIDTypographicFamily {
		return "", 0, false
	}
	var (
		key string
		ok  bool
	)
	score := 0
	switch platformID {
	case 3:

		if encodingID != 1 && encodingID != 10 {
			return "", 0, false
		}
		key, ok = windowsLocalizedNameLanguages[languageID]
		score = 10
	case 1:
		if encodingID != 0 {
			return "", 0, false
		}
		key, ok = macintoshLocalizedNameLanguages[languageID]
		score = 5
	default:
		return "", 0, false
	}
	if !ok || key == "" {
		return "", 0, false
	}
	if nameID == nameIDTypographicFamily {
		score += 2
	} else {
		score++
	}
	return key, score, true
}

func decodeNameString(platformID uint16, raw []byte) (string, bool) {
	if platformID == 1 {
		return decodeMacRoman(raw)
	}
	return decodeUTF16BE(raw)
}

func decodeUTF16BE(raw []byte) (string, bool) {
	if len(raw) < 2 || len(raw)%2 != 0 {
		return "", false
	}
	units := make([]uint16, len(raw)/2)
	for index := range units {
		units[index] = binary.BigEndian.Uint16(raw[index*2 : index*2+2])
	}
	return string(utf16.Decode(units)), true
}

func decodeMacRoman(raw []byte) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var builder strings.Builder
	builder.Grow(len(raw))
	for _, b := range raw {
		builder.WriteRune(charmap.Macintosh.DecodeByte(b))
	}
	return builder.String(), true
}

func normalizeNameValue(value string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(value), "\x00\uFEFF"))
}

func ttcFontOffsets(reader io.ReaderAt, size int64) []int64 {
	header := make([]byte, 12)
	if !readFullAt(reader, size, header, 0) {
		return nil
	}
	if header[0] != 't' || header[1] != 't' || header[2] != 'c' || header[3] != 'f' {
		return nil
	}
	numFonts := int64(binary.BigEndian.Uint32(header[8:12]))
	if numFonts <= 0 || numFonts > maxSFNTTables {
		return nil
	}
	if !validFontRange(12, numFonts*4, size) {
		return nil
	}
	buf := make([]byte, numFonts*4)
	if !readFullAt(reader, size, buf, 12) {
		return nil
	}
	offsets := make([]int64, numFonts)
	for index := range offsets {
		offsets[index] = int64(binary.BigEndian.Uint32(buf[index*4 : index*4+4]))
	}
	return offsets
}

func mergeLocalizedNames(into, from map[string]string) map[string]string {
	if len(from) == 0 {
		return into
	}
	if into == nil {
		into = make(map[string]string, len(from))
	}
	for key, value := range from {
		if value == "" {
			continue
		}
		if _, exists := into[key]; exists {
			continue
		}
		into[key] = value
	}
	if len(into) == 0 {
		return nil
	}
	return into
}

func validFontRange(offset, length, size int64) bool {
	if offset < 0 || length < 0 || size < 0 {
		return false
	}
	return offset <= size && length <= size-offset
}

func readFullAt(reader io.ReaderAt, size int64, buf []byte, offset int64) bool {
	if !validFontRange(offset, int64(len(buf)), size) {
		return false
	}
	n, _ := reader.ReadAt(buf, offset)
	return n == len(buf)
}

func isMonospace(parsed *sfnt.Font) bool {
	narrow, ok := glyphAdvance(parsed, monospaceProbeA)
	if !ok {
		return false
	}
	wide, ok := glyphAdvance(parsed, monospaceProbeB)
	if !ok {
		return false
	}
	return narrow == wide && narrow > 0
}

func glyphAdvance(parsed *sfnt.Font, r rune) (fixed.Int26_6, bool) {
	index, err := parsed.GlyphIndex(nil, r)
	if err != nil || index == 0 {
		return 0, false
	}
	advance, err := parsed.GlyphAdvance(nil, index, fixed.I(monospacePPEM), font.HintingNone)
	if err != nil {
		return 0, false
	}
	return advance, true
}

func hasNerdFontIcons(parsed *sfnt.Font) bool {
	hits := 0
	for _, r := range nerdFontProbeRunes {
		index, err := parsed.GlyphIndex(nil, r)
		if err != nil {
			continue
		}
		if index != 0 {
			hits++
		}
	}
	return hits >= nerdFontProbeThreshold
}

func existingFontDirectories() []string {
	directories := make([]string, 0, 4)
	for _, directory := range fontDirectories() {
		if strings.TrimSpace(directory) == "" {
			continue
		}
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			directories = append(directories, directory)
		}
	}
	return directories
}
