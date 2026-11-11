package windowsservice

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseServiceCommandLine(t *testing.T) {
	cases := []struct { name, input string; want []string }{
		{"quoted paths", `"C:\Program Files\Yggdrasil\yggdrasil.exe" -useconffile "C:\ProgramData\Yggdrasil\yggdrasil.conf"`, []string{`C:\Program Files\Yggdrasil\yggdrasil.exe`, "-useconffile", `C:\ProgramData\Yggdrasil\yggdrasil.conf`}},
		{"escaped quote", `app.exe -label "a\\\"b"`, []string{"app.exe", "-label", `a\"b`}},
		{"unquoted", `C:\Yggdrasil\yggdrasil.exe -autoconf`, []string{`C:\Yggdrasil\yggdrasil.exe`, "-autoconf"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseServiceCommandLine(tc.input)
			if err != nil { t.Fatal(err) }
			if !reflect.DeepEqual(got, tc.want) { t.Fatalf("got %#v, want %#v", got, tc.want) }
		})
	}
}

func TestParseServiceCommandLineRejectsUnterminatedQuote(t *testing.T) {
	if _, err := ParseServiceCommandLine(`app.exe "unfinished`); err == nil { t.Fatal("expected error") }
}

func TestResolveConfigPath(t *testing.T) {
	cases := []struct { args []string; want string }{
		{[]string{`C:\yggdrasil.exe`, "-useconffile", `C:\data\yggdrasil.conf`}, `C:\data\yggdrasil.conf`},
		{[]string{`C:\yggdrasil.exe`, `-useconffile=C:\data\yggdrasil.conf`}, `C:\data\yggdrasil.conf`},
		{[]string{"yggdrasil.exe", "-autoconf"}, ""},
		{[]string{"yggdrasil.exe", "-useconf"}, ""},
	}
	for _, tc := range cases {
		got, err := ResolveConfigPath(tc.args)
		if err != nil { t.Fatal(err) }
		if got != tc.want { t.Fatalf("got %q, want %q", got, tc.want) }
	}
	for _, args := range [][]string{{"yggdrasil.exe", "-useconffile"}, {"yggdrasil.exe", "-useconffile="}} {
		if _, err := ResolveConfigPath(args); err == nil { t.Fatalf("expected missing-path error for %#v", args) }
	}
}

func TestParseAdminListen(t *testing.T) {
	got, err := ParseAdminListen([]byte(`{ AdminListen: "tcp://127.0.0.1:9001" }`))
	if err != nil { t.Fatal(err) }
	if got != "tcp://127.0.0.1:9001" { t.Fatalf("unexpected endpoint %q", got) }
}

func TestParseAdminListenRejectsRemoteAndDisabled(t *testing.T) {
	for _, input := range []string{`{ AdminListen: "tcp://0.0.0.0:9001" }`, `{ AdminListen: "" }`, `{}`, `{ AdminListen: 42 }`} {
		if _, err := ParseAdminListen([]byte(input)); err == nil { t.Fatalf("expected error for %s", input) }
	}
}

func TestParseAdminListenMalformed(t *testing.T) {
	if _, err := ParseAdminListen([]byte(strings.Repeat("{", 10))); err == nil { t.Fatal("expected parse error") }
}
