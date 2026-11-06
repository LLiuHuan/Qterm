package hostimport

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/syndtr/goleveldb/leveldb/journal"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/table"
	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/nacl/secretbox"
)

var ErrTermiusDBMissing = errors.New("termius indexeddb not found")

var ErrTermiusKeyMissing = errors.New("termius local key not found in keychain")

func DefaultTermiusDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch {
	case isDarwin():
		return filepath.Join(home, "Library", "Application Support", "Termius", "IndexedDB", "file__0.indexeddb.leveldb")
	case isWindows():
		return filepath.Join(os.Getenv("APPDATA"), "Termius", "IndexedDB", "file__0.indexeddb.leveldb")
	default:
		if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
			candidate := filepath.Join(xdg, "Termius", "IndexedDB", "file__0.indexeddb.leveldb")
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
		}
		return filepath.Join(home, ".config", "Termius", "IndexedDB", "file__0.indexeddb.leveldb")
	}
}

func ParseTermiusDB(path string) (Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultTermiusDBPath()
	}

	key, err := loadTermiusLocalKey()
	if err != nil {
		return Result{}, err
	}
	return parseTermiusDBWithKey(path, key)
}

func ParseTermiusDBWithKey(path string, key [32]byte) (Result, error) {
	if path == "" {
		path = DefaultTermiusDBPath()
	}
	return parseTermiusDBWithKey(path, key)
}

func parseTermiusDBWithKey(path string, key [32]byte) (Result, error) {
	if !strings.HasSuffix(path, ".indexeddb.leveldb") {
		candidate := filepath.Join(path, "IndexedDB", "file__0.indexeddb.leveldb")
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		}
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return Result{}, ErrTermiusDBMissing
	}

	snapshotDir, cleanup, snapshotErr := snapshotLevelDB(path)
	if snapshotErr != nil {
		return Result{}, snapshotErr
	}
	defer cleanup()

	values, readErr := readTermiusRecordValues(snapshotDir)
	if readErr != nil {
		return Result{}, readErr
	}
	return prepareTermiusEntries(collectTermiusStore(values, key)), nil
}

func loadTermiusLocalKey() ([32]byte, error) {
	var zero [32]byte
	candidates := []struct{ service, account string }{
		{"Termius", "localKey"},
		{"Termius/localKey", "localKey"},
	}
	var lastErr error
	for _, candidate := range candidates {

		secret, err := darwinKeychainPassword(candidate.service, candidate.account)
		if err != nil || strings.TrimSpace(secret) == "" {

			secret, err = keyring.Get(candidate.service, candidate.account)
		}
		if err != nil || strings.TrimSpace(secret) == "" {
			lastErr = err
			continue
		}
		if key, ok := normalizeTermiusKey(secret); ok {
			return key, nil
		}
	}
	if lastErr == nil {
		lastErr = ErrTermiusKeyMissing
	}
	return zero, ErrTermiusKeyMissing
}

func normalizeTermiusKey(secret string) ([32]byte, bool) {
	var key [32]byte
	secret = strings.TrimSpace(secret)
	if decoded, err := base64.StdEncoding.DecodeString(secret); err == nil && len(decoded) == 32 {
		copy(key[:], decoded)
		return key, true
	}
	if len(secret) == 32 {
		copy(key[:], secret)
		return key, true
	}
	return key, false
}

type termiusRawRecord struct {
	sequence uint64
	deleted  bool
	value    []byte
}

func readTermiusRecordValues(dir string) ([][]byte, error) {
	records := make(map[string]termiusRawRecord, 4096)
	if err := readTermiusTables(dir, records); err != nil {
		return nil, err
	}
	if err := readTermiusJournals(dir, records); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := make([][]byte, 0, len(keys))
	for _, key := range keys {
		record := records[key]
		if record.deleted {
			continue
		}
		values = append(values, record.value)
	}
	return values, nil
}

func readTermiusTables(dir string, records map[string]termiusRawRecord) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.ldb"))
	if err != nil {
		return fmt.Errorf("list termius sstables: %w", err)
	}
	sort.Strings(files)
	for _, path := range files {
		if err := readTermiusTable(path, records); err != nil {
			return err
		}
	}
	return nil
}

func readTermiusTable(path string, records map[string]termiusRawRecord) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open termius sstable: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat termius sstable: %w", err)
	}

	reader, err := table.NewReader(file, info.Size(), storage.FileDesc{}, nil, nil, &opt.Options{Comparer: idbComparer})
	if err != nil {
		return fmt.Errorf("read termius sstable: %w", err)
	}
	iterator := reader.NewIterator(nil, nil)
	defer iterator.Release()
	for iterator.Next() {
		internalKey := iterator.Key()
		if len(internalKey) < 8 {
			continue
		}
		suffix := binary.LittleEndian.Uint64(internalKey[len(internalKey)-8:])
		userKey := string(internalKey[:len(internalKey)-8])
		mergeTermiusRecord(records, userKey, termiusRawRecord{
			sequence: suffix >> 8,
			deleted:  byte(suffix) == 0,
			value:    append([]byte(nil), iterator.Value()...),
		})
	}
	if err := iterator.Error(); err != nil {
		return fmt.Errorf("iterate termius sstable: %w", err)
	}
	return nil
}

func readTermiusJournals(dir string, records map[string]termiusRawRecord) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.log"))
	if err != nil {
		return fmt.Errorf("list termius journals: %w", err)
	}
	sort.Strings(files)
	for _, path := range files {
		if err := readTermiusJournal(path, records); err != nil {
			return err
		}
	}
	return nil
}

func readTermiusJournal(path string, records map[string]termiusRawRecord) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open termius journal: %w", err)
	}
	defer file.Close()

	reader := journal.NewReader(file, nil, false, true)
	for {
		record, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			return nil
		}
		if nextErr != nil {

			return nil
		}
		data, readErr := io.ReadAll(record)
		if readErr != nil {
			return nil
		}
		applyTermiusWriteBatch(data, records)
	}
}

func applyTermiusWriteBatch(data []byte, records map[string]termiusRawRecord) {
	if len(data) < 12 {
		return
	}
	sequence := binary.LittleEndian.Uint64(data[:8])
	count := int(binary.LittleEndian.Uint32(data[8:12]))
	cursor := 12
	for index := 0; index < count && cursor < len(data); index++ {
		kind := data[cursor]
		cursor++
		keyLength, consumed := readV8Varint(data[cursor:])
		if consumed == 0 || keyLength < 0 || cursor+consumed+keyLength > len(data) {
			return
		}
		cursor += consumed
		key := string(data[cursor : cursor+keyLength])
		cursor += keyLength
		switch kind {
		case 0:
			mergeTermiusRecord(records, key, termiusRawRecord{sequence: sequence + uint64(index), deleted: true})
		case 1:
			valueLength, consumedValue := readV8Varint(data[cursor:])
			if consumedValue == 0 || valueLength < 0 || cursor+consumedValue+valueLength > len(data) {
				return
			}
			cursor += consumedValue
			value := append([]byte(nil), data[cursor:cursor+valueLength]...)
			cursor += valueLength
			mergeTermiusRecord(records, key, termiusRawRecord{sequence: sequence + uint64(index), value: value})
		default:

			return
		}
	}
}

func mergeTermiusRecord(records map[string]termiusRawRecord, key string, record termiusRawRecord) {
	if existing, ok := records[key]; ok && existing.sequence >= record.sequence {
		return
	}
	records[key] = record
}

type termiusToken struct {
	text  string
	value int64
	isInt bool

	depth int
}

func parseTermiusTokens(raw []byte) []termiusToken {
	tokens := make([]termiusToken, 0, len(raw)/8+16)
	depth := 0
	index := 0
	for index < len(raw) {
		switch raw[index] {
		case '"':
			length, consumed := readV8Varint(raw[index+1:])
			if consumed == 0 {
				index++
				continue
			}
			start := index + 1 + consumed
			end := start + length
			if length < 0 || end > len(raw) {
				index++
				continue
			}
			if utf8.Valid(raw[start:end]) {
				tokens = append(tokens, termiusToken{text: string(raw[start:end]), depth: depth})
			}
			index = end
		case 'I':
			if value, consumed := readV8SignedInt(raw[index+1:]); consumed > 0 {
				tokens = append(tokens, termiusToken{value: value, isInt: true, depth: depth})
				index += 1 + consumed
				continue
			}
			index++
		case 'o', 'A', 'a':

			depth++
			index++
		case '{', '$':

			if depth > 0 {
				depth--
			}
			index++
		default:
			index++
		}
	}
	return tokens
}

func readV8Varint(raw []byte) (int, int) {
	value := 0
	shift := 0
	for index, b := range raw {
		if index >= 5 {
			return 0, 0
		}
		value |= int(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, index + 1
		}
		shift += 7
	}
	return 0, 0
}

func readV8SignedInt(raw []byte) (int64, int) {
	unsigned, consumed := readV8Varint(raw)
	if consumed == 0 {
		return 0, 0
	}
	value := int64(uint64(unsigned)>>1) ^ -int64(unsigned&1)
	return value, consumed
}

func decryptTermiusValue(value string, key [32]byte) (string, bool) {
	if len(value) < 40 || !strings.HasPrefix(value, "BA") {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) < 2+24+16 {
		return "", false
	}
	var nonce [24]byte
	copy(nonce[:], decoded[2:26])
	plaintext, ok := secretbox.Open(nil, decoded[26:], &nonce, &key)
	if !ok {
		return "", false
	}
	return string(plaintext), true
}

type termiusHostRecord struct {
	key          string
	localID      string
	aliases      []string
	address      string
	label        string
	username     string
	password     string
	port         int64
	identityRef  string
	sshConfigRef string
	groupRef     string
	updatedAt    string
}

type termiusIdentityRecord struct {
	key       string
	localID   string
	aliases   []string
	username  string
	password  string
	updatedAt string
}

type termiusSSHConfigRecord struct {
	key         string
	localID     string
	aliases     []string
	identityRef string
	port        int64
	updatedAt   string
}

type termiusGroupRecord struct {
	key       string
	localID   string
	aliases   []string
	label     string
	parentRef string
	updatedAt string
}

type termiusStore struct {
	hosts      []termiusHostRecord
	sshConfigs []termiusSSHConfigRecord
	identities []termiusIdentityRecord
	groups     []termiusGroupRecord
}

func collectTermiusStore(values [][]byte, key [32]byte) termiusStore {
	store := termiusStore{}
	for _, value := range values {
		record := parseTermiusTokens(value)
		if len(record) == 0 {
			continue
		}
		fields := termiusRecordFields(record, key)
		switch termiusRecordKind(fields, record) {
		case "host":
			host := termiusHostRecord{
				key:          termiusRecordKey(fields),
				localID:      strings.TrimSpace(fields["local_id"]),
				address:      fields["address"],
				label:        fields["label"],
				username:     fields["username"],
				password:     fields["password"],
				port:         termiusFieldInt(fields, "port"),
				identityRef:  termiusNestedRef(record, "identity"),
				sshConfigRef: termiusNestedRef(record, "ssh_config"),
				groupRef:     termiusNestedRef(record, "group"),
				updatedAt:    fields["updated_at"],
			}
			if host.identityRef == "" {
				host.identityRef = firstNonEmpty(fields["identity"], fields["identity_id"], fields["ssh_key"])
			}
			host.aliases = termiusRecordAliases(fields)
			if strings.TrimSpace(host.address) == "" && strings.TrimSpace(host.label) == "" {
				continue
			}
			store.hosts = append(store.hosts, host)
		case "ssh_config":
			config := termiusSSHConfigRecord{
				key:         termiusRecordKey(fields),
				localID:     strings.TrimSpace(fields["local_id"]),
				identityRef: termiusNestedRef(record, "identity"),
				port:        termiusFieldInt(fields, "port"),
				updatedAt:   fields["updated_at"],
			}
			config.aliases = termiusRecordAliases(fields)
			store.sshConfigs = append(store.sshConfigs, config)
		case "identity":
			identity := termiusIdentityRecord{
				key:       termiusRecordKey(fields),
				localID:   strings.TrimSpace(fields["local_id"]),
				username:  fields["username"],
				password:  fields["password"],
				updatedAt: fields["updated_at"],
			}
			if identity.username == "" && identity.password == "" {
				continue
			}
			identity.aliases = termiusRecordAliases(fields)
			store.identities = append(store.identities, identity)
		case "group":
			label := strings.TrimSpace(fields["label"])
			if label == "" {
				continue
			}
			group := termiusGroupRecord{
				key:       termiusRecordKey(fields),
				localID:   strings.TrimSpace(fields["local_id"]),
				label:     label,
				parentRef: termiusNestedRef(record, "parent_group"),
				updatedAt: fields["updated_at"],
			}
			if group.parentRef == "" {
				group.parentRef = firstNonEmpty(fields["parent_group"], fields["parent"], fields["parent_id"])
			}
			group.aliases = termiusRecordAliases(fields)
			store.groups = append(store.groups, group)
		}
	}
	return store
}

func termiusRecordFields(record []termiusToken, key [32]byte) map[string]string {
	fields := make(map[string]string, 16)
	depth := termiusRecordDepth(record)
	for index := 0; index+1 < len(record); index++ {
		name := record[index]
		if name.isInt || name.depth != depth || !termiusIsFieldName(name.text) {
			continue
		}
		next := record[index+1]
		if next.depth != depth {

			continue
		}
		if !next.isInt && (termiusIsFieldName(next.text) || termiusIsRecordType(next.text)) {

			continue
		}
		value := termiusTokenString(next)
		if !next.isInt {
			if decrypted, ok := decryptTermiusValue(next.text, key); ok {
				value = decrypted
			}
		}
		if _, exists := fields[name.text]; !exists {
			fields[name.text] = value
		}
	}
	return fields
}

func termiusRecordDepth(record []termiusToken) int {
	depth := 0
	for index, token := range record {
		if index == 0 || token.depth < depth {
			depth = token.depth
		}
	}
	return depth
}

func termiusRecordKind(fields map[string]string, record []termiusToken) string {
	uri := fields["resource_uri"]
	switch {
	case strings.Contains(uri, "/terminal/ssh/config/"):
		return "ssh_config"
	case strings.Contains(uri, "/terminal/host/"):
		return "host"
	case strings.Contains(uri, "/terminal/identity/"):
		return "identity"
	case strings.Contains(uri, "/terminal/group/"):
		return "group"
	case uri != "":

		return ""
	}
	if strings.TrimSpace(fields["address"]) != "" {
		return "host"
	}

	for index := 0; index+1 < len(record); index++ {
		if record[index].isInt || record[index].text != "ssh_config" || record[index+1].isInt {
			continue
		}
		if record[index+1].text == "id" {
			return "host"
		}
	}
	if fields["username"] != "" || fields["password"] != "" {
		return "identity"
	}
	if fields["label"] != "" && fields["parent_group"] != "" {
		return "group"
	}
	return ""
}

func termiusRecordKey(fields map[string]string) string {
	if localID := strings.TrimSpace(fields["local_id"]); localID != "" {
		return "local:" + localID
	}
	if id := strings.TrimSpace(fields["id"]); id != "" {
		return "id:" + id
	}
	return ""
}

func termiusRecordAliases(fields map[string]string) []string {
	aliases := make([]string, 0, 4)
	appendAlias := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range aliases {
			if existing == value {
				return
			}
		}
		aliases = append(aliases, value)
	}
	if uri := fields["resource_uri"]; uri != "" {
		appendAlias(uri)
		appendAlias(termiusResourceID(uri))
	}
	appendAlias(fields["local_id"])
	appendAlias(fields["id"])
	return aliases
}

func termiusResourceID(uri string) string {
	trimmed := strings.Trim(uri, "/")
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		return trimmed[index+1:]
	}
	return trimmed
}

func termiusNestedRef(record []termiusToken, field string) string {
	depth := termiusRecordDepth(record)
	for index := 0; index+2 < len(record); index++ {
		if record[index].isInt || record[index].depth != depth || record[index].text != field {
			continue
		}
		if record[index+1].isInt || record[index+1].depth != depth+1 || record[index+1].text != "id" {
			continue
		}
		value := strings.TrimSpace(termiusTokenString(record[index+2]))
		if value == "" || termiusIsFieldName(value) {
			continue
		}
		return value
	}
	return ""
}

func termiusFieldInt(fields map[string]string, name string) int64 {
	value := strings.TrimSpace(fields[name])
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func termiusTokenString(token termiusToken) string {
	if token.isInt {
		return strconv.FormatInt(token.value, 10)
	}
	return token.text
}

func termiusIsRecordType(value string) bool {
	switch value {
	case "ssh_config", "identity", "ssh_key", "group", "parent_group":
		return true
	}
	return false
}

func termiusIsFieldName(value string) bool {
	switch value {
	case "id", "local_id", "updated_at", "label", "address", "username", "password",
		"private_key", "passphrase", "group", "group_id", "identity", "identity_id",
		"ssh_key", "ssh_key_id", "key", "parent", "parent_id", "parent_group", "port",
		"resource_uri", "status":
		return true
	}
	return false
}

func termiusPasswordSource(password string) string {
	if strings.TrimSpace(password) != "" {
		return "saved"
	}
	return "prompt"
}

func termiusGroupPaths(records []termiusGroupRecord, index map[string]termiusGroupRecord) map[string]string {
	paths := make(map[string]string, len(records))
	const maxDepth = 8
	for _, record := range records {
		if record.key == "" {
			continue
		}
		segments := make([]string, 0, maxDepth)
		seen := make(map[string]struct{}, maxDepth)
		current := record
		for depth := 0; depth < maxDepth; depth++ {
			label := strings.TrimSpace(current.label)
			if label == "" {
				break
			}
			segments = append(segments, label)
			if _, visited := seen[current.key]; visited {
				break
			}
			seen[current.key] = struct{}{}
			parentRef := strings.TrimSpace(current.parentRef)
			if parentRef == "" {
				break
			}
			parent, ok := index[parentRef]
			if !ok || parent.key == "" {
				break
			}
			current = parent
		}

		for left, right := 0, len(segments)-1; left < right; left, right = left+1, right-1 {
			segments[left], segments[right] = segments[right], segments[left]
		}
		paths[record.key] = strings.Join(segments, "/")
	}
	return paths
}

func prepareTermiusEntries(store termiusStore) Result {
	configs := indexTermiusRecords(store.sshConfigs, func(item termiusSSHConfigRecord) []string { return item.aliases }, func(item termiusSSHConfigRecord) string { return item.updatedAt })
	identities := indexTermiusRecords(store.identities, func(item termiusIdentityRecord) []string { return item.aliases }, func(item termiusIdentityRecord) string { return item.updatedAt })
	groups := indexTermiusRecords(store.groups, func(item termiusGroupRecord) []string { return item.aliases }, func(item termiusGroupRecord) string { return item.updatedAt })
	groupPaths := termiusGroupPaths(store.groups, groups)

	hosts := dedupeTermiusRecords(store.hosts, func(item termiusHostRecord) string { return item.key }, func(item termiusHostRecord) string { return item.updatedAt })
	result := Result{Entries: make([]Entry, 0, len(hosts))}
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		address := strings.TrimSpace(host.address)
		if address == "" {

			result.Skipped++
			continue
		}
		config, hasConfig := configs[host.sshConfigRef]
		if !hasConfig && host.localID != "" {

			config, hasConfig = configs[host.localID]
		}
		identity, hasIdentity := termiusResolveIdentity(host, config, hasConfig, identities)

		user := strings.TrimSpace(host.username)
		if user == "" && hasIdentity {
			user = strings.TrimSpace(identity.username)
		}
		password := host.password
		if password == "" && hasIdentity {
			password = identity.password
		}
		port := host.port
		if port == 0 && hasConfig {
			port = config.port
		}
		if port <= 0 || port > 65535 {
			port = 22
		}
		entry := Entry{
			Alias:          firstNonEmpty(host.label, address),
			HostName:       address,
			User:           user,
			Port:           int(port),
			Password:       password,
			PasswordSource: termiusPasswordSource(password),
			Group:          groupPaths[groups[host.groupRef].key],
		}
		dedupeKey := entry.HostName + "|" + entry.User + "|" + strconv.Itoa(entry.Port)
		if _, duplicate := seen[dedupeKey]; duplicate {
			continue
		}
		seen[dedupeKey] = struct{}{}
		result.Entries = append(result.Entries, entry)
	}
	for index := range result.Entries {
		result.Entries[index].Line = index + 1
	}
	return result
}

func termiusResolveIdentity(host termiusHostRecord, config termiusSSHConfigRecord, hasConfig bool, identities map[string]termiusIdentityRecord) (termiusIdentityRecord, bool) {
	references := []string{host.identityRef}
	if hasConfig {
		references = append(references, config.identityRef)
	}
	references = append(references, host.localID)
	if hasConfig {
		references = append(references, config.localID)
	}
	for _, reference := range references {
		reference = strings.TrimSpace(reference)
		if reference == "" {
			continue
		}
		if identity, ok := identities[reference]; ok {
			return identity, true
		}
	}
	return termiusIdentityRecord{}, false
}

func indexTermiusRecords[T any](items []T, aliases func(T) []string, updated func(T) string) map[string]T {
	index := make(map[string]T, len(items))
	for _, item := range items {
		for _, alias := range aliases(item) {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			if existing, ok := index[alias]; !ok || updated(item) >= updated(existing) {
				index[alias] = item
			}
		}
	}
	return index
}

func dedupeTermiusRecords[T any](items []T, key func(T) string, updated func(T) string) []T {
	latest := make(map[string]T, len(items))
	for _, item := range items {
		recordKey := key(item)
		if recordKey == "" {
			continue
		}
		if existing, ok := latest[recordKey]; !ok || updated(item) >= updated(existing) {
			latest[recordKey] = item
		}
	}
	result := make([]T, 0, len(latest))
	for _, item := range latest {
		result = append(result, item)
	}
	return result
}

func snapshotLevelDB(source string) (string, func(), error) {
	target, err := os.MkdirTemp("", "qterm-termius-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create termius snapshot dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(target) }
	entries, err := os.ReadDir(source)
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("read termius leveldb: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "LOCK" {
			continue
		}
		if !strings.HasPrefix(name, "MANIFEST-") &&
			!strings.HasSuffix(name, ".ldb") &&
			!strings.HasSuffix(name, ".log") &&
			name != "CURRENT" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(source, name))
		if readErr != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("copy termius leveldb file: %w", readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(target, name), data, 0o600); writeErr != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("write termius snapshot file: %w", writeErr)
		}
	}
	return target, cleanup, nil
}

type idbComparerType struct{}

func (idbComparerType) Compare(a, b []byte) int { return bytes.Compare(a, b) }
func (idbComparerType) Name() string            { return "idb_cmp1" }
func (idbComparerType) Separator(dst, a, b []byte) []byte {
	n := len(a)
	if n > len(b) {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	if i >= n {
		if len(a) < len(b) {
			return append(dst, a...)
		}
		return append(dst, b...)
	}
	if a[i] < b[i] {
		return append(dst, a[:i+1]...)
	}
	return append(dst, b[:i+1]...)
}
func (idbComparerType) Successor(dst, b []byte) []byte {
	for i := 0; i < len(b); i++ {
		if b[i] != 0xff {
			dst = append(dst, b[:i+1]...)
			dst[len(dst)-1]++
			return dst
		}
	}
	return append(dst, b...)
}

var idbComparer = idbComparerType{}
