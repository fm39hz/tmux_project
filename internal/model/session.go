package model

type Session struct {
	Name    string
	Cwd     string
	Windows []Window
}

type Window struct {
	Idx    int
	Name   string
	Cwd    string
	Layout string
	Panes  []Pane
}

type Pane struct {
	Idx int
	Cwd string
	Cmd string
}

type Usage struct {
	Name     string
	Opens    int64
	Kills    int64
	LastOpen int64
	LastKill int64
}
