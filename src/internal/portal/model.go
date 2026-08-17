package portal

// Selection tracks which environment is active per app, by index.
//
// Held apart from Config so the config stays an immutable view of the
// Nix payload and both front ends share one notion of "which environment am I
// looking at".
type Selection struct {
	byApp map[string]int
}

// NewSelection selects each app's first declared environment.
func NewSelection(apps []App) *Selection {
	selection := &Selection{byApp: make(map[string]int, len(apps))}
	for _, app := range apps {
		selection.byApp[app.Name] = 0
	}
	return selection
}

// Index returns the selected environment index for an app.
func (s *Selection) Index(app App) int {
	index := s.byApp[app.Name]
	if index < 0 || index >= len(app.Environments) {
		return 0
	}
	return index
}

// Current returns the selected environment, and false when the app declares none.
func (s *Selection) Current(app App) (Environment, bool) {
	if len(app.Environments) == 0 {
		return Environment{}, false
	}
	return app.Environments[s.Index(app)], true
}

// Cycle advances an app's environment by delta, wrapping in both directions.
func (s *Selection) Cycle(app App, delta int) {
	count := len(app.Environments)
	if count == 0 {
		return
	}
	next := (s.Index(app) + delta) % count
	if next < 0 {
		next += count
	}
	s.byApp[app.Name] = next
}

// Set selects an environment by name, reporting whether the app declares it.
func (s *Selection) Set(app App, name string) bool {
	for index, env := range app.Environments {
		if env.Name == name {
			s.byApp[app.Name] = index
			return true
		}
	}
	return false
}

// Row is one rendered line: an app, its selected environment, and that
// environment's status. Both front ends build their view from these.
type Row struct {
	App         App
	Environment Environment
	Status      Status
	// EnvironmentIndex and EnvironmentCount drive the "2/3" affordance that
	// tells a reader more environments exist.
	EnvironmentIndex int
	EnvironmentCount int
}

// Rows resolves the current view. Apps with no environments are skipped: a
// launcher row with nothing to launch is noise.
func Rows(apps []App, selection *Selection, statuses map[string]Status) []Row {
	rows := make([]Row, 0, len(apps))
	for _, app := range apps {
		env, ok := selection.Current(app)
		if !ok {
			continue
		}
		rows = append(rows, Row{
			App:              app,
			Environment:      env,
			Status:           statuses[Key(app.Name, env.Name)],
			EnvironmentIndex: selection.Index(app),
			EnvironmentCount: len(app.Environments),
		})
	}
	return rows
}
