package main

import (
	"58boss/boss"
	"58boss/initial"
	"58boss/job58"
	"58boss/save"
	"58boss/sqlite"
	"github.com/astaxie/beego/logs"
	"sync"
	"time"
)

var wg sync.WaitGroup

func main() {
	logs.Reset()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)

	initial.Init()
	sqlite.DbInit()

	logs.Info("Ready to open browsers")
	time.Sleep(5 * time.Second)

	wg.Add(1)
	go boss.Run(&wg)

	wg.Add(1)
	go job58.Run(&wg)

	wg.Add(1)
	go save.Run(&wg)

	wg.Wait()
}
