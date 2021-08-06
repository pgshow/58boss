package main

import (
	"58boss/initial"
	"58boss/job58"
	"58boss/save"
	"58boss/sqlite"
	"sync"
)

var wg sync.WaitGroup

func main() {
	initial.Init()
	sqlite.DbInit()

	//wg.Add(1)
	//go boss.Run(&wg)

	wg.Add(1)
	go job58.Run(&wg)

	wg.Add(1)
	go save.Run(&wg)

	wg.Wait()
}
