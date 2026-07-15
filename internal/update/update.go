// Package update implements harharbinks' opt-in update mechanism. It is the only
// package in harharbinks that accesses the network; every other package runs
// completely offline. Update checks are strictly opt-in (see Enabled), so the
// default experience makes no network request at all. When enabled, Check asks
// GitHub for the latest release — caching the answer for a day — purely to notify
// the user, and Apply (only ever driven by the explicit `hhb update` command)
// downloads and atomically replaces the running binary after verifying its
// checksum. Keeping all of this behind one package makes the network surface easy
// to audit and to disable.
package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"

	"github.com/bapatchirag/harharbinks/internal/config"
)

// repoSlug is the GitHub "owner/name" that harharbinks releases are published to;
// Check and Apply query its releases.
const repoSlug = "bapatchirag/harharbinks"

// checksumsFile is the goreleaser-published asset listing SHA-256 sums for every
// release archive. Apply verifies the downloaded archive against it before
// replacing the binary.
const checksumsFile = "checksums.txt"

// cacheTTL is how long a fetched "latest version" result is trusted before Check
// makes a new network request. Within this window Check answers from the on-disk
// cache and touches no network, so at most one request is made per day.
const cacheTTL = 24 * time.Hour

// envVar opts in to (or out of) update checks for a single run, overriding the
// persisted config.UpdateCheck setting.
const envVar = "HHB_UPDATE_CHECK"

// cacheFile is the base name of the update-check cache, stored beside the config
// file in ~/.config/hhb.
const cacheFile = "update.json"

// Result summarizes a version check: the running version, the latest published
// release, its release-page URL, and whether the latter is newer than the former.
type Result struct {
	// Current is the running harharbinks version, as passed by the caller.
	Current string
	// Latest is the newest published release version (empty if none was found).
	Latest string
	// URL is the latest release's page, for the user to review.
	URL string
	// Newer reports whether Latest is a strictly higher version than Current.
	Newer bool
}

// Enabled reports whether update checks are turned on. Checks are opt-in: they
// stay off unless the user sets config.UpdateCheck or the HHB_UPDATE_CHECK
// environment variable. The environment variable takes precedence over the
// persisted setting so a single run can enable ("1", "true") or disable ("0",
// "false") checking regardless of the saved config; a present but unparseable
// value is treated as opting in.
func Enabled(cfg config.Config) bool {
	if v, ok := os.LookupEnv(envVar); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
		return strings.TrimSpace(v) != ""
	}
	return cfg.UpdateCheck
}

// devMarker matches version strings produced by non-release builds: a `git
// describe` value with commits after the last tag (e.g. "v1.0.1-3-gabc1234"), a
// "-dirty" working tree, or a Go module pseudo-version's timestamp+hash suffix
// (e.g. "0.0.0-20260705004817-2cc9a8fe1146").
var devMarker = regexp.MustCompile(`-\d+-g[0-9a-f]+|-dirty|\d{14}-[0-9a-f]{12}`)

// IsReleaseBuild reports whether version denotes a pristine, published release
// rather than a local or development build. Only release builds trigger the
// launch-time update notification and may be self-updated, so a developer working
// on unreleased code is never nagged and `hhb update` never overwrites a
// locally-built binary. A version qualifies only when it is clean semver
// ("v1.2.3", or a real pre-release such as "v1.2.3-rc1") carrying none of the
// development markers above; anything else — "dev", "(devel)", a dirty tree, a
// commits-after-tag describe, or a pseudo-version — is a development build. When
// in doubt it returns false, biasing toward silence.
func IsReleaseBuild(version string) bool {
	v := strings.TrimSpace(version)
	if v == "" || v == "dev" || v == "(devel)" {
		return false
	}
	if devMarker.MatchString(v) {
		return false
	}
	_, err := semver.NewVersion(strings.TrimPrefix(v, "v"))
	return err == nil
}

// state is the persisted update-check cache, stored next to the config file so
// launch checks can answer from disk without hitting the network more than once
// per cacheTTL.
type state struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
	URL           string    `json:"url"`
}

// cachePath returns the absolute path of the update-check cache file,
// ~/.config/hhb/update.json.
func cachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFile), nil
}

// loadState reads the cache. It is best-effort: a missing, unreadable, or
// malformed cache yields the zero state and ok=false, so a bad cache simply
// forces a fresh check rather than failing.
func loadState() (state, bool) {
	path, err := cachePath()
	if err != nil {
		return state{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state{}, false
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return state{}, false
	}
	return s, true
}

// saveState writes the cache as indented JSON, creating ~/.config/hhb if needed.
// It is best-effort; a write failure only means the next launch re-checks.
func saveState(s state) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// isNewer reports whether latest is a strictly higher semantic version than
// current. A leading "v" on either is ignored. If either fails to parse as semver
// (for example current is a development build), it returns false.
func isNewer(current, latest string) bool {
	cur, err := semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(current), "v"))
	if err != nil {
		return false
	}
	lat, err := semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(latest), "v"))
	if err != nil {
		return false
	}
	return lat.GreaterThan(cur)
}

// result assembles a Result for the given running version and discovered latest
// release.
func result(current, latest, url string) Result {
	return Result{
		Current: current,
		Latest:  latest,
		URL:     url,
		Newer:   isNewer(current, latest),
	}
}

// newUpdater builds a go-selfupdate updater that verifies every download against
// the release's checksums.txt before applying it.
func newUpdater() (*selfupdate.Updater, error) {
	return selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: checksumsFile},
	})
}

// Check reports whether a newer release than current exists. Unless force is set,
// it answers from the ~/.config/hhb/update.json cache while that cache is younger
// than cacheTTL, making no network request; otherwise it queries GitHub for the
// latest release and refreshes the cache. It is best-effort: any error resolving
// the cache path, reaching GitHub, or parsing the response is returned so the
// launch path can silently ignore it. The provided context bounds the network
// request, so callers should pass a timeout.
func Check(ctx context.Context, current string, force bool) (Result, error) {
	if !force {
		if s, ok := loadState(); ok && s.LatestVersion != "" && time.Since(s.CheckedAt) < cacheTTL {
			return result(current, s.LatestVersion, s.URL), nil
		}
	}
	up, err := newUpdater()
	if err != nil {
		return Result{Current: current}, err
	}
	rel, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return Result{Current: current}, err
	}
	if !found || rel == nil {
		// No published release; record the check time so we do not immediately retry.
		_ = saveState(state{CheckedAt: time.Now()})
		return Result{Current: current}, nil
	}
	_ = saveState(state{CheckedAt: time.Now(), LatestVersion: rel.Version(), URL: rel.URL})
	return result(current, rel.Version(), rel.URL), nil
}

// Cached returns the last known check result from the on-disk cache without ever
// touching the network, so headless commands can surface an already-known update
// without the cost or side effects of a request. ok is false when no usable
// cached result exists. It ignores cacheTTL: a stale hint is acceptable and is
// refreshed the next time the TUI or `hhb update` runs.
func Cached(current string) (Result, bool) {
	s, ok := loadState()
	if !ok || s.LatestVersion == "" {
		return Result{}, false
	}
	return result(current, s.LatestVersion, s.URL), true
}

// Apply downloads the latest release's archive for the running OS/architecture,
// verifies it against the release checksums, and atomically replaces the running
// executable. It is only ever called from the explicit `hhb update` command and
// returns a Result describing what was installed. When no newer release exists it
// makes no change and returns a Result with Newer false. Callers are responsible
// for refusing to run on development builds (see IsReleaseBuild); Apply always
// targets whatever binary is currently running.
func Apply(ctx context.Context, current string) (Result, error) {
	up, err := newUpdater()
	if err != nil {
		return Result{Current: current}, err
	}
	rel, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return Result{Current: current}, err
	}
	if !found || rel == nil {
		return Result{Current: current}, nil
	}
	res := result(current, rel.Version(), rel.URL)
	if !res.Newer {
		return res, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return res, err
	}
	// Resolve symlinks so we replace the real binary rather than a link to it.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if err := up.UpdateTo(ctx, rel, exe); err != nil {
		return res, err
	}
	return res, nil
}
