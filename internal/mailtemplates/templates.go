package mailtemplates

import (
	"embed"
	"html/template"
	"io/fs"
	"net/url"
	"path"

	"github.com/glasskube/cloud/internal/types"

	"github.com/glasskube/cloud/internal/env"
)

//go:embed templates/*
var embeddedFS embed.FS

var templates *template.Template
var funcMap = template.FuncMap{
	"QueryEscape": url.QueryEscape,
}

func init() {
	if fsys, err := fs.Sub(embeddedFS, "templates"); err != nil {
		panic(err)
	} else {
		templates = template.Must(parse(fsys, "*.html", "fragments/*.html"))
	}
}

func parse(fsys fs.FS, patterns ...string) (*template.Template, error) {
	var t *template.Template = template.New("").Funcs(funcMap)
	for _, p := range patterns {
		if files, err := fs.Glob(fsys, p); err != nil {
			return nil, err
		} else {
			for _, file := range files {
				// funcMap must be present during parsing *and* execution
				if ft, err := template.New("").Funcs(funcMap).ParseFS(fsys, file); err != nil {
					return nil, err
				} else if _, err := t.AddParseTree(file, ft.Lookup(path.Base(file)).Tree); err != nil {
					return nil, err
				}
			}
		}
	}
	return t, nil
}

func Welcome() (*template.Template, any) {
	return templates.Lookup("welcome.html"),
		map[string]any{
			"Host": env.Host(),
		}
}

func InviteUser(
	userAccount types.UserAccount,
	organization types.Organization,
	token string,
) (*template.Template, any) {
	return templates.Lookup("invite-user.html"),
		map[string]any{
			"UserAccount":  userAccount,
			"Organization": organization,
			"Host":         env.Host(),
			"Token":        token,
		}
}

func InviteCustomer(
	userAccount types.UserAccount,
	organization types.Organization,
	token string,
	applicationName string,
) (*template.Template, any) {
	return templates.Lookup("invite-customer.html"),
		map[string]any{
			"UserAccount":     userAccount,
			"Organization":    organization,
			"ApplicationName": applicationName,
			"Host":            env.Host(),
			"Token":           token,
		}
}

func VerifyEmail(userAccount types.UserAccount, token string) (*template.Template, any) {
	return templates.Lookup("verify-email-registration.html"), map[string]any{
		"UserAccount": userAccount,
		"Host":        env.Host(),
		"Token":       token,
	}
}

func PasswordReset(userAccount types.UserAccount, token string) (*template.Template, any) {
	return templates.Lookup("password-reset.html"), map[string]any{
		"UserAccount": userAccount,
		"Host":        env.Host(),
		"Token":       token,
	}
}
