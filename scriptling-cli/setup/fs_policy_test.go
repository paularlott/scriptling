package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// The CLI wires --allowed-paths into every library that reads local files:
// wait_for and the messaging uploads were missing it, so a script could use
// them to probe or exfiltrate files outside the jail (strix 4250 vuln-0001).
// Each refusal happens before any network call, and the script cannot catch
// it.
func TestAllowedPathsReachFileReadingLibraries(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	jail := filepath.Join(base, "jail")
	secret := filepath.Join(base, "secret.txt")
	if err := os.MkdirAll(jail, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("TOP-SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	for name, call := range map[string]string{
		"wait_for.file":         `import scriptling.wait_for as w; w.file("` + secret + `", timeout=0)`,
		"wait_for.dir":          `import scriptling.wait_for as w; w.dir("` + base + `", timeout=0)`,
		"wait_for.file_content": `import scriptling.wait_for as w; w.file_content("` + secret + `", "TOP", timeout=0)`,
		"discord.send_file":     `import scriptling.messaging.discord as m; m.send_file(m.client("token"), "123", "` + secret + `")`,
		"slack.send_file":       `import scriptling.messaging.slack as m; m.send_file(m.client("xoxb-1", "xapp-1"), "C1", "` + secret + `")`,
		"telegram.send_file":    `import scriptling.messaging.telegram as m; m.send_file(m.client("1:token"), "42", "` + secret + `")`,
	} {
		p := scriptling.New()
		Scriptling(p, nil, false, []string{jail}, nil, nil, logslog.New(logslog.Config{Level: "error"}), "", "")
		p.EnableOutputCapture()
		_, err := p.Eval("try:\n    " + strings.ReplaceAll(call, "; ", "\n    ") + "\nexcept Exception:\n    pass\nprint('continued')")
		if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
			t.Errorf("%s: expected access denied, got %v", name, err)
		}
		if strings.Contains(p.GetOutput(), "continued") {
			t.Errorf("%s: the script caught the denial and carried on", name)
		}
	}
}

// The CLI applies --network-policy to every messaging library, the platforms'
// own API hosts included: a script supplies the bot token, so those hosts are
// egress channels like any other (strix 4250 vuln-0006). The refusal comes
// before any connection.
func TestNetworkPolicyReachesMessagingPlatformHosts(t *testing.T) {
	policy := &netsecurity.Config{AllowHosts: []string{"only.example"}}
	for name, call := range map[string]string{
		"telegram": `import scriptling.messaging.telegram as m; m.send_message(m.client("1:tok"), "42", "x")`,
		"discord":  `import scriptling.messaging.discord as m; m.send_message(m.client("tok"), "123", "x")`,
		"slack":    `import scriptling.messaging.slack as m; m.send_message(m.client("xoxb-1", "xapp-1"), "C1", "x")`,
	} {
		p := scriptling.New()
		Scriptling(p, nil, false, nil, nil, nil, logslog.New(logslog.Config{Level: "error"}), "", "", policy)
		_, err := p.Eval(strings.ReplaceAll(call, "; ", "\n"))
		if err == nil || !strings.Contains(err.Error(), "network policy") {
			t.Errorf("%s: expected a network policy refusal, got %v", name, err)
		}
	}
}
