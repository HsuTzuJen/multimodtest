package main

import "git.insea.io/booyah/server/kakarot/pkg/clog"

func main() {
	clog.InitLog(clog.Config{Path: "./log/test.log", Level: "debug"})
}
