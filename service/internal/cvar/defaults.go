package cvar

const (
	TypeString = "string"
	TypeInt    = "int"
	TypeBool   = "bool"

	CategorySite     = "Site"
	CategoryFeatures = "Features"
	CategoryTheme    = "Theme"
)

type Def struct {
	Key, MainType, Title, Description, Category, ValueString string
	Ordinal, ValueInt                                         int
}

func Defaults(siteTitle string) []Def {
	if siteTitle == "" {
		siteTitle = "RepoManager"
	}
	return []Def{
		{
			Key: "site_title", MainType: TypeString, ValueString: siteTitle,
			Title: "Site title", Description: "Shown in the header and browser tab.",
			Category: CategorySite, Ordinal: 10,
		},
		{
			Key: "show_footer", MainType: TypeBool, ValueInt: 1,
			Title: "Show footer", Description: "Display the page footer in the web UI.",
			Category: CategorySite, Ordinal: 20,
		},
		{
			Key: "show_version_number", MainType: TypeBool, ValueInt: 1,
			Title: "Show version number", Description: "Display the installed version in the footer.",
			Category: CategorySite, Ordinal: 30,
		},
		{
			Key: "theme_color_scheme_switcher_enabled", MainType: TypeBool, ValueInt: 1,
			Title: "Color scheme switcher", Description: "Allow users to switch light/dark/auto.",
			Category: CategoryTheme, Ordinal: 10,
		},
		{
			Key: "theme_name", MainType: TypeString, ValueString: "",
			Title: "Theme name", Description: "Named CSS theme (empty = default).",
			Category: CategoryTheme, Ordinal: 20,
		},
		{
			Key: "theme_control", MainType: TypeString, ValueString: "user",
			Title: "Theme control", Description: "Who may change theme: user or system.",
			Category: CategoryTheme, Ordinal: 30,
		},
	}
}
