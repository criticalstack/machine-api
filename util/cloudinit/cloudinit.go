package cloudinit

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	machinev1 "github.com/criticalstack/machine-api/api/v1alpha1"
)

const (
	nodeCloudInit = `## template: jinja
#cloud-config
{{template "files" .Files}}
runcmd:
{{- template "commands" .PreCritCommands }}
  - 'crit up --config /var/lib/crit/config.yaml {{.Verbosity}}'
{{- template "commands" .PostCritCommands }}
{{- template "ntp" .NTP }}
{{- template "users" .Users }}
`
	commandsTemplate = `{{- define "commands" -}}
{{ range . }}
  - {{printf "%q" .}}
{{- end -}}
{{- end -}}
`
	filesTemplate = `{{ define "files" -}}
write_files:{{ range . }}
-   path: {{.Path}}
    {{ if ne .Encoding "" -}}
    encoding: "{{.Encoding}}"
    {{ end -}}
    {{ if ne .Owner "" -}}
    owner: {{.Owner}}
    {{ end -}}
    {{ if ne .Permissions "" -}}
    permissions: '{{.Permissions}}'
    {{ end -}}
    content: |
{{.Content | Indent 6}}
{{- end -}}
{{- end -}}
`
	ntpTemplate = `{{ define "ntp" -}}
{{- if . }}
ntp:
  {{ if .Enabled -}}
  enabled: true
  {{ end -}}
  servers:{{ range .Servers }}
    - {{ . }}
  {{- end -}}
{{- end -}}
{{- end -}}
`
	usersTemplate = `{{ define "users" -}}
{{- if . }}
users:{{ range . }}
  - name: {{ .Name }}
    {{- if .Passwd }}
    passwd: {{ .Passwd }}
    {{- end -}}
    {{- if .Gecos }}
    gecos: {{ .Gecos }}
    {{- end -}}
    {{- if .Groups }}
    groups: {{ .Groups }}
    {{- end -}}
    {{- if .HomeDir }}
    homedir: {{ .HomeDir }}
    {{- end -}}
    {{- if .Inactive }}
    inactive: true
    {{- end -}}
    {{- if .LockPassword }}
    lock_passwd: {{ .LockPassword }}
    {{- end -}}
    {{- if .Shell }}
    shell: {{ .Shell }}
    {{- end -}}
    {{- if .PrimaryGroup }}
    primary_group: {{ .PrimaryGroup }}
    {{- end -}}
    {{- if .Sudo }}
    sudo: {{ .Sudo }}
    {{- end -}}
    {{- if .SSHAuthorizedKeys }}
    ssh_authorized_keys:{{ range .SSHAuthorizedKeys }}
      - {{ . }}
    {{- end -}}
    {{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}
`
)

type Config struct {
	Files            []machinev1.File
	PreCritCommands  []string
	PostCritCommands []string
	Users            []machinev1.User
	NTP              *machinev1.NTP
	Format           machinev1.Format
	Verbosity        bool
}

func Write(input *Config) ([]byte, error) {
	tm := template.New("Node").Funcs(template.FuncMap{
		"trim": strings.TrimSpace,
		"Indent": func(i int, input string) string {
			split := strings.Split(input, "\n")
			ident := "\n" + strings.Repeat(" ", i)
			return strings.Repeat(" ", i) + strings.Join(split, ident)
		},
	})
	if _, err := tm.Parse(filesTemplate); err != nil {
		return nil, errors.Wrap(err, "failed to parse files template")
	}

	if _, err := tm.Parse(commandsTemplate); err != nil {
		return nil, errors.Wrap(err, "failed to parse commands template")
	}

	if _, err := tm.Parse(ntpTemplate); err != nil {
		return nil, errors.Wrap(err, "failed to parse ntp template")
	}

	if _, err := tm.Parse(usersTemplate); err != nil {
		return nil, errors.Wrap(err, "failed to parse users template")
	}

	t, err := tm.Parse(nodeCloudInit)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s template", "Node")
	}

	var out bytes.Buffer
	if err := t.Execute(&out, input); err != nil {
		return nil, errors.Wrapf(err, "failed to generate %s template", "Node")
	}
	return out.Bytes(), nil
}
