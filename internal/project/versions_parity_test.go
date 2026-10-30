package project

import "strings"

// Version resolution cases for TestModelMatchesProjectPy: pin files of each
// format, support files, as-of dates with floors and precision, releasesOf,
// "selects": false, installed versions and the environment they give.

// versionDefinitions are fixture definitions shaped like the real ones,
// with installed.sh scripts printing toolchains under the project's
// toolchains/ directory (they run from the root).
var versionDefinitions = map[string]string{
	"py/config.json": `{"abbreviation": "py", "dirNames": ["python"], "version": "=3.15.1",
		"pinFiles": [{"file": ".python-version", "format": "python-version"},
			{"file": ".tool-versions", "format": "tool-versions", "key": "python"},
			{"file": "mise.toml", "format": "mise", "key": "python"}],
		"supportFiles": [{"file": "pyproject.toml", "format": "python-classifiers"},
			{"file": "setup.cfg", "format": "python-classifiers"},
			{"file": ".github/workflows/*.y*ml", "format": "github-setup", "key": "python-version"}],
		"oldestInstallable": "3.8", "asOfPrecision": "line",
		"path": ["{home}/bin", "~/.local/bin"],
		"env": {"UV_PYTHON": "{home}/bin/python3", "PY_LINE": "{major}.{minor}", "BRACES": "{{x}}"}}`,
	"py/releases.json": `{"releases": [["3.7.9", "2020-08-17"], ["3.8.10", "2021-05-03"], ["3.9.5", "2021-05-03"],
		["3.10.0", "2021-10-04"], ["3.11.0", "2022-10-24"], ["3.12.0", "2023-10-02"], ["3.12.4", "2024-06-06"],
		["3.13.0", "2024-10-07"], ["3.14.0", "2025-10-07"], ["3.15.1", "2026-09-01"]]}`,
	"py/installed.sh": `printf '3.12.4\t%s/toolchains/py312\n' "$PWD"
printf '3.15.0\t%s/toolchains/py315\n' "$PWD"
printf '3.11.2\t~/py311\n'
printf 'not-a-version\t/x\n'
printf '3.10.1\n'
printf ' 3.9.1 \t %s/toolchains/py39 \n' "$PWD"
printf '3.12.4\t%s/toolchains/py312b\n' "$PWD"`,

	"jv/config.json": `{"version": "=25.0.5", "track": "lts", "releaseLine": 1,
		"pinFiles": [{"file": ".java-version", "format": "java-version"},
			{"file": ".sdkmanrc", "format": "sdkmanrc", "key": "java"},
			{"file": "gradle/gradle-daemon-jvm.properties", "format": "gradle-daemon-jvm"}],
		"supportFiles": [{"file": "pom.xml", "format": "maven-source-level"},
			{"file": ".github/workflows/*.y*ml", "format": "github-setup", "key": "java-version"}],
		"oldestInstallable": "11", "path": ["{home}/b*n", "{home}/*", "{home}/.h*"], "env": {"JAVA_HOME": "{home}"}}`,
	"jv/releases.json": `{"releases": [["8.0.292", "2021-04-20", "lts"], ["11.0.11", "2021-04-20", "lts"],
		["16.0.1", "2021-04-20"], ["17.0.1", "2021-10-27", "lts"], ["21.0.2", "2024-01-16", "lts"],
		["22.0.1", "2024-04-16"], ["25.0.5", "2026-07-21", "lts"]]}`,
	"jv/installed.sh": `printf '17.0.2\t%s/toolchains/jdk17\n21.0.2\t%s/toolchains/jdk21\n25.0.1\t%s/toolchains/jdk25\n' "$PWD" "$PWD" "$PWD"`,

	"net/config.json": `{"version": "=10.0.403", "track": "lts", "releaseLine": 3,
		"pinFiles": [{"file": "global.json", "format": "global-json"}],
		"supportFiles": [{"file": ".github/workflows/*.y*ml", "format": "github-setup", "key": "dotnet-version", "selects": false}],
		"floorFiles": [{"file": "*.csproj", "format": "target-framework"}],
		"oldestInstallable": "6.0", "path": ["{home}"], "env": {"DOTNET_ROOT": "{home}"}}`,
	"net/releases.json": `{"releases": [["5.0.400", "2021-08-10"], ["6.0.100", "2021-11-08", "lts"],
		["8.0.100", "2023-11-14", "lts"], ["9.0.100", "2024-11-12"], ["10.0.403", "2026-10-14", "lts"]]}`,
	"net/installed.sh": `printf '8.0.100\t%s/toolchains/net\n10.0.403\t%s/toolchains/net\n10.0.300\t%s/toolchains/net\n' "$PWD" "$PWD" "$PWD"`,
	"fs/config.json":   `{"version": "=10.0.403", "releasesOf": "net", "releaseLine": 3, "pinFiles": [{"file": "global.json", "format": "global-json"}]}`,

	"rs/config.json": `{"version": "=1.99.1", "pinFiles": [{"file": "rust-toolchain.toml", "format": "rust-toolchain-toml"},
		{"file": "rust-toolchain", "format": "rust-toolchain"}]}`,
	"rs/releases.json": `{"releases": [["1.50.0", "2021-02-11"], ["1.56.1", "2021-11-01"], ["1.80.1", "2024-08-08"], ["1.99.1", "2026-09-30"]]}`,
	"gol/config.json":  `{"version": "=1.27.3", "pinFiles": [{"file": "go.mod", "format": "go-mod-toolchain"}], "path": ["{home}/bin"]}`,
	"gol/installed.sh": `printf '1.27.1\t%s/toolchains/go\n1.26.0\t%s/toolchains/go\n' "$PWD" "$PWD"`,
	"ph/config.json": `{"version": "=8.5.12", "asOfPrecision": "line",
		"pinFiles": [{"file": ".php-version", "format": "php-version"}, {"file": "composer.json", "format": "composer-json"}]}`,
	"ph/releases.json": `{"releases": [["5.6.40", "2019-01-10"], ["7.4.20", "2021-06-03"], ["8.1.0", "2021-11-25"], ["8.5.12", "2026-09-30"]]}`,
	"nd/config.json": `{"version": "=24.22.1", "track": "lts", "releaseLine": 1,
		"pinFiles": [{"file": ".nvmrc", "format": "nvmrc"}, {"file": ".node-version", "format": "node-version"},
			{"file": ".tool-versions", "format": "tool-versions", "key": "nodejs"}]}`,
	"nd/releases.json": `{"releases": [["14.17.0", "2021-05-11", "lts"], ["16.1.0", "2021-05-04"], ["16.17.0", "2022-08-16", "lts"], ["24.22.1", "2026-10-01", "lts"]]}`,
	"ts/config.json":   `{"releasesOf": "nd", "version": "=24.22.1", "track": "lts", "releaseLine": 1, "pinFiles": [{"file": ".nvmrc", "format": "nvmrc"}]}`,
	"eng/config.json": `{"version": "=24.22.1", "track": "lts", "releaseLine": 1, "path": ["{home}/bin"],
		"pinFiles": [{"file": ".nvmrc", "format": "nvmrc"}],
		"rangeFiles": [{"file": "package.json", "format": "npm-engines", "key": "node"}]}`,
	"eng/installed.sh":  `printf '26.13.0\t%s/toolchains/node26\n20.1.0\t%s/toolchains/node20\n22.4.0-rc.1\t/opt/rc\n' "$PWD" "$PWD"`,
	"eng/releases.json": `{"releases": [["14.17.0", "2021-05-11", "lts"], ["24.22.1", "2026-10-01", "lts"]]}`,
	"eng24/config.json": `{"version": "=24.22.1", "releaseLine": 1, "releasesOf": "eng",
		"rangeFiles": [{"file": "package.json", "format": "npm-engines"}]}`,
	"eng24/installed.sh": `printf '24.1.0\t/opt/n24\n26.13.0\t/opt/n26\n'`,
	"zg/config.json":     `{"version": "=0.17.0", "pinFiles": [{"file": "build.zig.zon", "format": "build-zig-zon"}], "path": ["{home}"]}`,
	"zg/releases.json":   `{"releases": [["0.13.0", "2024-06-07"], ["0.14.0", "2025-03-05"], ["0.14.1", "2025-05-21"], ["0.17.0", "2026-10-01"]]}`,
	"zg/installed.sh":    `printf '0.17.0\t%s/toolchains/zig17\n0.14.1\t%s/toolchains/zig14\n' "$PWD" "$PWD"`,
	"rb/config.json":     `{"version": "=4.0.9", "pinFiles": [{"file": ".ruby-version", "format": "ruby-version"}]}`,
	"sw/config.json":     `{"version": "=6.2", "pinFiles": [{"file": ".swift-version", "format": "swift-version"}]}`,

	"ex/config.json": `{"version": "=1.20.4",
		"pinFiles": [{"file": ".tool-versions", "format": "tool-versions", "key": "elixir", "strip": "-otp-[0-9]+$"},
			{"file": "mise.toml", "format": "mise", "key": "elixir", "strip": "-otp-[0-9]+$"}]}`,
	"exall/config.json":    `{"version": "=1.0", "pinFiles": [{"file": "pin", "format": "python-version", "strip": "[0-9.]+"}]}`,
	"badstrip/config.json": `{"version": "=1.0", "pinFiles": [{"file": "pin", "format": "python-version", "strip": "("}]}`,
	"inexact/config.json":  `{"version": "3.1"}`,
	"numeric/config.json":  `{"version": 3.1}`,
	"badfmt/config.json":   `{"version": "=1.0", "pinFiles": [{"file": "pin", "format": "nope"}]}`,
	"badspec/config.json":  `{"version": "=abc"}`,
	"noline/config.json":   `{"version": "=2.5.1", "releaseLine": null, "path": ["{home}/bin"]}`,
	"noline/installed.sh":  `printf '2.5.0\t/opt/x\n'`,
	"neg/config.json":      `{"version": "=2.5.1", "releaseLine": -2}`,
	"failing/config.json":  `{"version": "=1.0", "path": ["{home}/bin"]}`,
	"failing/installed.sh": `printf '1.0\t/opt/x\n'; exit 3`,
	"oddpath/config.json":  `{"version": "=1.0", "path": ["{home}/{nope}"]}`,
	"rel/config.json":      `{"path": ["static*/bin", "toolchains"]}`,
	"dup/config.json":      `{"path": ["/usr/bin", "{home}/bin"]}`,
	"also/config.json":     `{"path": ["/usr/bin", "/bin"]}`,
	"off/config.json":      `{"path": ["/usr/sbin"]}`,
	"oddpath/installed.sh": `printf '1.0\t/opt/x\n'`,
}

// versionLangs are the fixture languages compared.
var versionLangs = []string{"eng", "eng24", "py", "jv", "net", "fs", "rs", "gol", "ph", "nd", "ts", "zg", "rb", "sw", "ex", "exall", "badstrip",
	"inexact", "numeric", "badfmt", "badspec", "noline", "neg", "failing", "oddpath", "rel", "dup", "also", "off", "unknown"}

// pinned projects: one component per pin file format.
var pinnedFiles = map[string]string{
	"a/python/.python-version":                     "# comment\n\n  3.12  \n",
	"b/python/.tool-versions":                      "nodejs 20\npython 3.11.4 # pinned\n",
	"c/python/mise.toml":                           "[env]\npython = \"9\"\n[tools]\nnode = '20'\npython = [\"3.10\", \"3.9\"]\n",
	"d/python/.python-version":                     "system\n",
	"e/python/mise.toml":                           "[tools]\npython = { version = \"3.13.1\", virtualenv = \".venv\" }\n",
	"a/jv/.java-version":                           "temurin64-21.0.2\n",
	"b/jv/.sdkmanrc":                               "# sdkman\njava=17.0.2-tem\n",
	"c/jv/gradle/gradle-daemon-jvm.properties":     "toolchainVendor=ADOPTIUM\ntoolchainVersion=21\n",
	"d/jv/.java-version":                           "openjdk64\n",
	"a/net/global.json":                            `{"sdk": {"version": "8.0.100", "rollForward": "latestFeature"}}`,
	"b/net/global.json":                            `{"sdk": {"version": "8.0.100", "rollForward": "disable"}}`,
	"c/net/global.json":                            `{"sdk": {"version": "9.0.100"}}`,
	"d/net/global.json":                            `{"sdk": null, "msbuild-sdks": {}}`,
	"e/net/global.json":                            `{"sdk": {"version": "11.0.100-rc.1.1", "rollForward": "latestMajor"}}`,
	"a/fs/global.json":                             `{"sdk": {"version": "8.0.100", "rollForward": "latestMinor"}}`,
	"b/fs/global.json":                             `{"sdk": {"version": "8.0.100", "rollForward": 7}}`,
	"c/fs/global.json":                             `// comments\n{"sdk": {"version": "8.0.100"}}`,
	"d/fs/global.json":                             `{"sdk": {"version": null}}`,
	"a/rs/rust-toolchain.toml":                     "[toolchain]\nchannel = \"1.80.1\"\ncomponents = [\"clippy\"]\n",
	"b/rs/rust-toolchain":                          "stable\n",
	"c/rs/rust-toolchain":                          "[toolchain]\nchannel = '1.75'\n",
	"d/rs/rust-toolchain.toml":                     "[toolchain]\ncomponents = [\"clippy\"]\n",
	"a/gol/go.mod":                                 "module x\r\n\r\ngo 1.22\r\ntoolchain go1.22.3\r\n",
	"b/gol/go.mod":                                 "module x\n\ngo 1.22\n",
	"c/gol/go.mod":                                 "module x\ntoolchain\ngo1.23.0\n",
	"a/ph/composer.json":                           `{"config": {"platform": {"php": "8.1.2"}}}`,
	"b/ph/.php-version":                            "8.3\n",
	"c/ph/composer.json":                           `{"config": {"platform": {"php": 7.40}}}`,
	"d/ph/composer.json":                           `{"require": {"php": "^8.1"}}`,
	"e/ph/composer.json":                           `{"config": []}`,
	"a/nd/.nvmrc":                                  "lts/*\n",
	"b/nd/.node-version":                           "v20.11.0\n",
	"c/nd/.nvmrc":                                  "  'v18'  \n",
	"a/ts/.nvmrc":                                  "16\n",
	"a/zg/build.zig.zon":                           ".{\n    .name = .a,\n    .minimum_zig_version = \"0.14.0\",\n}\n",
	"b/zg/build.zig.zon":                           ".{\n    // .minimum_zig_version = \"0.14.0\",\n}\n",
	"c/zg/build.zig.zon":                           ".{ .minimum_zig_version = \"0.15.0-dev.1149+4e6a04929\" }",
	"d/zg/build.zig.zon":                           ".{\n  .minimum_zig_version = \"master\",\n}\n",
	"toolchains/zig14/":                            "",
	"a/rb/.ruby-version":                           "ruby-3.2.2\n",
	"b/rb/.ruby-version":                           "3.3.0-preview1\n",
	"a/sw/.swift-version":                          "5.10\n",
	".tool-versions":                               "nodejs 22.1.0\n",
	".hecatron/languages/py/config.json":           `{"pinFiles": [{"file": ".pyver", "format": "python-version"}]}`,
	"e/python/.pyver":                              "3.9\n",
	".hecatron/languages/badfmt/config.json":       `{}`,
	"a/badfmt/pin":                                 "1.0",
	"toolchains/py312/bin/":                        "",
	"toolchains/py312b/bin/":                       "",
	"toolchains/py315/bin/":                        "",
	"toolchains/jdk17/bin/":                        "",
	"toolchains/jdk21/bin/":                        "",
	"toolchains/net/":                              "",
	"static1/bin/":                                 "",
	"static2/bin/":                                 "",
	".static/bin/":                                 "",
	"toolchains/go/bin/":                           "",
	"b/gol/sub/":                                   "",
	"f/python/subdir/":                             "",
	"f/python/.python-version":                     "\ufeff3.12\n",
	"f/jv/.sdkmanrc":                               "java = 21.0.2-amzn\n",
	"f/net/global.json":                            "\ufeff{\"sdk\": {\"version\": \"8.0.100\"}}",
	"f/gol/go.mod":                                 "module x\n  toolchain   go1.21.0  \n",
	"f/rs/rust-toolchain":                          "1.80\n",
	"f/nd/.nvmrc":                                  "node\n",
	"f/rb/.ruby-version":                           "\n# none\n",
	"f/ph/composer.json":                           `{"config": {"platform": {"php": true}}}`,
	"f/ts/.nvmrc":                                  "v16.20.2\r\n",
	"f/sw/.swift-version":                          "6.0-dev\n",
	"g/python/.python-version":                     "3.12 3.11\n",
	"g/jv/.java-version":                           "17 \n",
	"g/net/global.json":                            `{"sdk": {"version": "8.0.100", "rollForward": "latestPatch"}, "sdk": {"version": "6.0.100"}}`,
	"g/ph/composer.json":                           `{"config": {"platform": {"php": "8"}}}`,
	"g/rs/rust-toolchain.toml":                     "[toolchain]\n# channel = \"1.70\"\nchannel = \"nightly-2024-01-01\"\n",
	"g/nd/.node-version":                           "20 # pinned\n",
	"g/gol/go.mod":                                 "toolchain go1.21rc2\n",
	"h/python/pyproject.toml":                      "",
	"h/net/global.json":                            `{"sdk": {"version": 8}}`,
	"h/fs/global.json":                             `{"sdk": "8.0.100"}`,
	"i/net/global.json":                            `[1, 2]`,
	"i/fs/global.json":                             `{"sdk": {"version": "8.0.100", "rollForward": ["x"]}}`,
	"i/jv/.sdkmanrc":                               "java=21.0.2-tem.x_1\nmaven=3\n",
	"i/python/.tool-versions":                      "python\npython  3.8.10  3.9\n",
	"a/ex/.tool-versions":                          "erlang 27.1\nelixir 1.17.3-otp-27\n",
	"b/ex/mise.toml":                               "[tools]\nelixir = \"1.18-otp-26\"\n",
	"c/ex/.tool-versions":                          "elixir 1.19.1\n",
	"d/ex/.tool-versions":                          "elixir 1.16.0-rc.1\n",
	"e/ex/.tool-versions":                          "elixir ref:main-otp-27\n",
	"f/ex/.tool-versions":                          "elixir v1.15.8-otp-26\n",
	"a/exall/pin":                                  "1.2.3\n",
	"a/badstrip/pin":                               "1.2.3\n",
	"i/rs/rust-toolchain.toml":                     "[toolchain]\nchannel = [\"1.70\"]\n",
	"i/ph/composer.json":                           `{"config": {"platform": {"php": "v8.2.1"}}}`,
	"i/gol/go.mod":                                 "go 1.21\ntoolchain local\n",
	".github/workflows/README":                     "",
	"j/python/.tool-versions":                      "",
	"j/jv/.java-version":                           "",
	"j/net/global.json":                            ``,
	"k/python/.python-version/":                    "",
	"k/jv/.java-version":                           "1.8\n",
	"k/rs/rust-toolchain":                          "1.80.1 # stable\n",
	"l/python/mise.toml":                           "[tools]\n\"python\" = \"3.11\"\n",
	"l/jv/.sdkmanrc":                               "java=\n",
	"l/net/global.json":                            `{"sdk": {"version": "8.0.100", "rollForward": null}}`,
	"l/fs/global.json":                             `{"sdk": {"version": "8.0.100", "rollForward": "latestFeature"}, "x": NaN}`,
	".hecatron/languages/rb/config.json":           `{"version": "3.3"}`,
	".hecatron/languages/sw/config.json":           `{"version": "6"}`,
	".hecatron/languages/gol/config.json":          `{"version": "1.26"}`,
	".hecatron/languages/oddpath/config.json":      `{}`,
	".hecatron/languages/projectonly/config.json":  `{"version": "2.0", "path": ["{home}/bin"]}`,
	".hecatron/languages/projectonly/installed.sh": `printf '2.0.1\t/opt/p\n2.1\t/opt/q\n'`,
	"a/projectonly/":                               "",
	".hecatron/languages/net/config.json":          `{"supportFiles": [{"file": "support.txt", "format": "python-version"}]}`,
	"m/net/support.txt":                            "7.0\n",
	"n/net/support.txt":                            "10.0.300\n",
	"o/net/support.txt":                            "12\n",
	"m/python/setup.cfg":                           "classifiers =\n    Programming Language :: Python :: 3.9\n    Programming Language :: Python :: 3.10\n",
	"n/python/pyproject.toml":                      "classifiers = [\"Programming Language :: Python :: 3.14\", \"Programming Language :: Python :: 3.7\"]\n",
	"o/python/pyproject.toml":                      "\"Programming Language :: Python :: 3.11é\",\n\"Programming Language :: Python :: 3.12-x\",\n\"Programming Language :: Python :: 3.13.1\"",
	"m/jv/pom.xml":                                 "<properties><maven.compiler.source>1.6</maven.compiler.source><maven.compiler.target> 1.8 </maven.compiler.target></properties>",
	"n/jv/pom.xml":                                 "<release>7</release><source>11</source>",
	"o/jv/pom.xml":                                 "<source>${java.version}</source><target>17</target>",
}

func versionConfig(components ...string) string {
	return `{"schema": 1, "components": ["` + strings.Join(components, `", "`) + `"],
		"languages": ["py", "jv", "net", "fs", "rs", "gol", "ph", "nd", "ts", "zg", "rb", "sw", "ex", "exall", "badstrip", "badfmt", "projectonly"]}`
}

var pinnedComponents = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}

// supportFiles: CI workflows and build files saying which versions a
// component supports, at the root and in components.
var supportFiles = map[string]string{
	".hecatron/config.json": `{"schema": 1, "components": ["a", "b", "c"], "languages": ["py", "jv", "net", "fs", "nd"]}`,
	".github/workflows/ci.yml": `jobs:
  test:
    strategy:
      matrix:
        python: ['3.9', "3.11", 3.x]
        java: [8, 11]
        dotnet:
          - 8.0.x
          - 9.0.x
    steps:
      - uses: actions/setup-python@v5
        with:
          python-version: ${{ matrix.python }}
      - uses: actions/setup-java@v4
        with:
          java-version: ${{matrix.java}} # the matrix
      - uses: actions/setup-dotnet@v4
        with:
          dotnet-version: |
            ${{ matrix.dotnet }}
      - with:
          dotnet-version:
            - 6.0.x
            - '7.0.100'
`,
	".github/workflows/release.yaml": "      - with:\n          java-version: '1.8'\n          python-version: \"3.10\"\n",
	".github/workflows/.hidden.yml":  "python-version: 3.13\n",
	"a/py/x.py":                      "",
	"a/jv/pom.xml":                   "<source>1.7</source>",
	"a/net/x.csproj":                 "",
	"b/python/pyproject.toml":        "classifiers = ['Programming Language :: Python :: 3.7']",
	"b/jv/.github/workflows/w.yml":   "java-version: [ '21', 17 ]\n",
	"b/net/.github/workflows/w.yml":  "dotnet-version: 10.0.300\n",
	"b/fs/.github/workflows/w.yml":   "dotnet-version: '9.0'\n",
	"c/net/.github/workflows/w.yml":  "dotnet-version: lts\n",
	"c/python/setup.cfg":             "Programming Language :: Python :: 3.16",
}

func versionCases() []parityCase {
	pinned := with(pinnedFiles, map[string]string{".hecatron/config.json": versionConfig(pinnedComponents...)})
	langs := append(versionLangs, "projectonly")
	var cases []parityCase
	add := func(name string, files map[string]string, env map[string]string) {
		cases = append(cases, parityCase{name: name, files: files, builtin: versionDefinitions, langs: langs, env: env, versions: true})
	}
	add("versions: pins of each format", pinned, nil)
	add("versions: pins, from a subdirectory", pinned, nil)
	cases[len(cases)-1].cwd = "b/gol/sub"
	add("versions: support files", supportFiles, nil)
	for _, date := range []string{"2021-06-01", "2015-01-01", "2023-03-15", "2021-05-03", "2026-12-31"} {
		add("versions: pins as of "+date, pinned, map[string]string{"HECATRON_AS_OF": date})
		add("versions: support files as of "+date, supportFiles, map[string]string{"HECATRON_AS_OF": date})
	}
	for _, date := range []string{"2021-02-30", "0000-01-01", "20210101", "yesterday", "2021-W01"} {
		add("versions: HECATRON_AS_OF="+date, supportFiles, map[string]string{"HECATRON_AS_OF": date})
	}
	add("versions: global.json floors", map[string]string{
		".hecatron/config.json":  `{"schema": 1, "components": ["a", "b", "c", "d", "e", "f"], "languages": ["net", "fs"]}`,
		"a/net/global.json":      `{"sdk": {"version": "8.0.100", "rollForward": "latestMajor"}}`,
		"b/net/global.json":      `{"sdk": {"version": "8.0.100", "rollForward": "latestMajor"}}`,
		"b/net/src/A/A.csproj":   "<TargetFrameworks>netstandard2.0;net8.0;net11.0</TargetFrameworks>",
		"b/net/src/B/B.csproj":   "<TargetFramework>net9.0</TargetFramework><TargetFramework>net472</TargetFramework>",
		"c/net/global.json":      `{"sdk": {"version": "11.0.100", "rollForward": "latestMajor"}}`,
		"d/net/global.json":      `{"sdk": {"version": "8.0.100", "rollForward": "latestMajor"}}`,
		"d/net/bin/x.csproj":     "<TargetFramework>net12.0</TargetFramework>",
		"d/net/a/b/c/d/x.csproj": "<TargetFramework>net12.0</TargetFramework>",
		"d/net/a/b/c/x.csproj":   "<TargetFramework>net9.0</TargetFramework>",
		"e/net/global.json":      `{"sdk": {"version": "8.0.100", "rollForward": "latestMinor"}}`,
		"e/net/x.csproj":         "<TargetFramework>net10.0</TargetFramework>",
		"f/net/global.json":      `{"sdk": {"version": "9.0.100", "rollForward": "latestMajor"}}`,
		"f/net/x.csproj":         "<TargetFramework>net10.0</TargetFramework>",
		"a/fs/global.json":       `{"sdk": {"version": "8.0.100", "rollForward": "latestMajor"}}`,
		"a/fs/x.csproj":          "<TargetFramework>net12.0</TargetFramework>",
	}, nil)
	ranges := map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m"],
			"languages": ["eng", "eng24"]}`,
		"a/eng/package.json":     `{"engines": {"node": ">=22"}}`,
		"b/eng/package.json":     `{"engines": {"node": "^18 || ^20.1"}}`,
		"c/eng/package.json":     `{"engines": {"node": "  >= 16   <21 "}}`,
		"d/eng/package.json":     `{"engines": {"npm": ">=10"}}`,
		"d/package.json":         `{"engines": {"node": "20 - 26.13"}}`,
		"e/eng/package.json":     `{"engines": {"node": "lts"}}`,
		"f/eng/package.json":     `{"engines": {"node": 22}}`,
		"g/eng/package.json":     `{"engines": ">=22"}`,
		"h/eng/package.json":     `[1]`,
		"i/eng/package.json":     `{"engines": {"node": ">=22"`,
		"j/eng/package.json":     `{"engines": {"node": "~26.13.0 || 1.x"}}`,
		"j/eng/.nvmrc":           "26\n",
		"k/eng/package.json":     `{"engines": {"node": ">22.4"}}`,
		"l/eng/package.json":     `{"engines": {"node": "*"}, "engines": {"node": "<=20"}}`,
		"m/eng/package.json":     `{"engines": {"node": ">=27"}}`,
		"a/eng24/package.json":   `{"engines": {"node": ">=22"}}`,
		"toolchains/node26/bin/": "",
	}
	add("versions: engines ranges", ranges, nil)
	add("versions: engines ranges as of 2021", ranges, map[string]string{"HECATRON_AS_OF": "2021-06-01"})
	add("versions: global.json floors as of 2024", map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["a"], "languages": ["net"]}`,
		"a/net/global.json":     `{"sdk": {"version": "8.0.100", "rollForward": "latestMajor"}}`,
	}, map[string]string{"HECATRON_AS_OF": "2024-06-01"})
	add("versions: no HOME", pinned, map[string]string{"HOME": ""})
	add("versions: other languages' static paths", with(pinnedFiles, map[string]string{
		".hecatron/config.json": `{"schema": 1, "languages": ["py", "off", "dup", "also", "gol"]}`,
		".hecatron/local.json":  `{"disabled": ["off"]}`}), nil)
	add("versions: implPath outside the root", with(pinnedFiles, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["a", "b"], "implPath": "../{component}/{lang}", "languages": ["py"]}`}), nil)
	add("versions: project versions", with(config(`, "languages": ["py", "jv", "rs"]`), map[string]string{
		".hecatron/languages/py/config.json": `{"version": ""}`,
		".hecatron/languages/jv/config.json": `{"version": "=17.0.2"}`,
		".hecatron/languages/rs/config.json": `{"version": "1.80", "pinFiles": [{"file": "rust-toolchain", "format": "rust-toolchain"}]}`,
		"rs/rust-toolchain":                  "1.79\n",
		"jv/.java-version":                   "17\n",
	}), nil)
	add("versions: project version not a version", with(config(`, "languages": ["jv"]`), map[string]string{
		".hecatron/languages/jv/config.json": `{"version": "latest"}`}), nil)
	add("versions: project version a number", with(config(`, "languages": ["jv"]`), map[string]string{
		".hecatron/languages/jv/config.json": `{"version": 17}`}), nil)
	add("versions: project version null", with(config(`, "languages": ["jv"]`), map[string]string{
		".hecatron/languages/jv/config.json": `{"version": null}`}), nil)
	add("versions: odd pinFiles", with(config(`, "languages": ["jv"]`), map[string]string{
		".hecatron/languages/jv/config.json": `{"pinFiles": "x"}`}), nil)
	add("versions: non-ASCII digits", with(config(`, "languages": ["py"]`), map[string]string{
		"python/.python-version": "٣.12\n"}), nil)
	add("versions: as of a project version", with(config(`, "languages": ["py", "jv", "nd"]`), map[string]string{
		".hecatron/languages/py/config.json": `{"version": "3.7"}`,
		".hecatron/languages/jv/config.json": `{"version": "8"}`,
		".hecatron/languages/nd/config.json": `{"version": "16"}`,
		"python/x.py":                        "",
	}), map[string]string{"HECATRON_AS_OF": "2021-06-01"})
	cases = append(cases, parityCase{name: "versions: an old commit", git: true, commit: "2021-06-01",
		files:   with(supportFiles, map[string]string{"py/.python-version": "3.9\n"}),
		builtin: versionDefinitions, langs: langs, env: map[string]string{"HECATRON_AS_OF": ""}, versions: true})
	cases = append(cases, parityCase{name: "versions: a recent commit", git: true, commit: "2026-10-01",
		files: supportFiles, builtin: versionDefinitions, langs: langs, env: map[string]string{"HECATRON_AS_OF": ""}, versions: true})
	cases = append(cases, parityCase{name: "versions: no commit", git: true,
		files: supportFiles, builtin: versionDefinitions, langs: langs, env: map[string]string{"HECATRON_AS_OF": ""}, versions: true})
	// The real definitions, with the toolchains this machine has.
	real := with(pinnedFiles, map[string]string{
		".hecatron/config.json": `{"schema": 1, "components": ["a", "b", "c"], "implPath": "{component}/{lang}",
			"languages": ["python", "java", "go", "rust", "csharp", "javascript", "typescript", "ruby", "php"]}`,
		"a/python/.python-version":   "3.12\n",
		"b/python/pyproject.toml":    "classifiers = ['Programming Language :: Python :: 3.11']\n",
		"a/java/.java-version":       "21\n",
		"b/java/pom.xml":             "<source>1.7</source>",
		"a/go/go.mod":                "module x\ntoolchain go1.22.3\n",
		"b/go/go.mod":                "module x\n",
		"a/rust/rust-toolchain.toml": "[toolchain]\nchannel = \"stable\"\n",
		"a/csharp/global.json":       `{"sdk": {"version": "8.0.100", "rollForward": "latestFeature"}}`,
		"b/csharp/x.csproj":          "",
		"a/javascript/.nvmrc":        "20\n",
		"a/typescript/package.json":  "{}",
		"b/ruby/.ruby-version":       "3.3.0\n",
		"a/php/composer.json":        `{"config": {"platform": {"php": "8.2.0"}}}`,
		".github/workflows/ci.yml":   "dotnet-version: 9.0.x\nnode-version: 22\njava-version: 17\n",
	})
	realLangs := []string{"python", "java", "go", "rust", "csharp", "fsharp", "vbnet", "javascript", "typescript", "ruby", "php", "kotlin", "swift", "nope"}
	for _, asOf := range []string{"off", "2021-06-01", "2016-01-01"} {
		cases = append(cases, parityCase{name: "versions: real definitions as of " + asOf, files: real, langs: realLangs,
			env: map[string]string{"HECATRON_AS_OF": asOf}, versions: true})
	}
	return cases
}
