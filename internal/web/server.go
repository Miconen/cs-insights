package web

import (
	"cs-insights/internal/db"
	"html/template"
	"log"
	"net/http"
)

type Server struct {
	db *db.Database
}

func NewServer(database *db.Database) *Server {
	return &Server{db: database}
}

func (s *Server) Start(addr string) error {
	http.HandleFunc("/", s.handleDashboard)

	log.Printf("Starting web server on http://%s", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	playerName := r.URL.Query().Get("player")
	if playerName == "" {
		// Provide a simple default view or ask for player name
		tmpl := template.Must(template.New("index").Parse(indexTmpl))
		tmpl.Execute(w, nil)
		return
	}

	insights, err := s.db.GetInsightsForPlayer(playerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		PlayerName string
		Insights   []db.Insight
	}{
		PlayerName: playerName,
		Insights:   insights,
	}

	tmpl := template.Must(template.New("dashboard").Parse(dashboardTmpl))
	tmpl.Execute(w, data)
}

const indexTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CS Insights</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 h-screen flex items-center justify-center">
    <div class="bg-white p-8 rounded-lg shadow-md w-96">
        <h1 class="text-2xl font-bold mb-4">CS Insights Dashboard</h1>
        <form action="/" method="GET" class="space-y-4">
            <div>
                <label for="player" class="block text-sm font-medium text-gray-700">Enter Player Name</label>
                <input type="text" name="player" id="player" class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border" placeholder="e.g. s1mple">
            </div>
            <button type="submit" class="w-full flex justify-center py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500">
                View Insights
            </button>
        </form>
    </div>
</body>
</html>
`

const dashboardTmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Insights: {{.PlayerName}}</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-50 text-gray-900 font-sans">
    <div class="max-w-4xl mx-auto py-10 px-4 sm:px-6 lg:px-8">
        <div class="mb-8">
            <h1 class="text-3xl font-bold">Performance Insights</h1>
            <p class="text-gray-500">Analysis for <span class="font-semibold">{{.PlayerName}}</span></p>
        </div>

        <div class="space-y-4">
            {{if not .Insights}}
                <div class="bg-white shadow rounded-lg p-6 text-center text-gray-500">
                    No insights found for this player yet. Run the CLI tool to parse a demo first.
                </div>
            {{else}}
                {{range .Insights}}
                    <div class="bg-white shadow rounded-lg p-4 border-l-4 
                        {{if eq .Severity "High"}}border-red-500{{else if eq .Severity "Medium"}}border-yellow-500{{else}}border-blue-500{{end}}">
                        <div class="flex justify-between items-start">
                            <div>
                                <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium 
                                    {{if eq .Severity "High"}}bg-red-100 text-red-800{{else if eq .Severity "Medium"}}bg-yellow-100 text-yellow-800{{else}}bg-blue-100 text-blue-800{{end}}">
                                    {{.Type}}
                                </span>
                                <h3 class="mt-2 text-lg font-medium">{{.Description}}</h3>
                            </div>
                            <div class="text-sm text-gray-500 text-right">
                                <div>Round {{.Round}}</div>
                                <div class="text-xs">Tick {{.Tick}}</div>
                            </div>
                        </div>
                    </div>
                {{end}}
            {{end}}
        </div>
    </div>
</body>
</html>
`
