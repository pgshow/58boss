package util

import (
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/eddycjy/fake-useragent"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"
)

// GetList 从文件获取
func GetList(file string) []string {
	data, err := ioutil.ReadFile(file)

	if err != nil {

		panic(err)
	}

	return strings.Split(string(data), "\n")
}

// ListDrop 去掉注释和空白的行
func ListDrop(tmpWords []string) (words []string) {
	for _, s := range tmpWords {
		if s == "" || strings.HasPrefix(s,"#") {
			continue
		}
		words = append(words, s)
	}
	return
}

// GetRandomUA 随机获取 userAgent
func GetRandomUA() string {
	return browser.Computer()
}

// ContainAny 判断目标字符串是否在列表里面
func ContainAny(str string, list []string) bool {
	for _, n := range list {
		if strings.Contains(str, n) {
			return true
		}
	}
	return false
}

// RandSecond 随机秒数
func randSecond(min, max int64) time.Duration {
	var s int64
	if min >= max || min == 0 || max == 0 {
		s = max
		return time.Duration(max) * time.Second
	}
	s = rand.Int63n(max-min)+min

	logs.Info(fmt.Sprintf("wait for %d seconds", s))
	return time.Duration(s) * time.Second
}

// RandSleep 随机延迟
func RandSleep(min, max int64)  {
	time.Sleep(randSecond(min, max))
}

// ShortComName 公司名称简化
func ShortComName(name string) (shorted string) {
	shorted = strings.Replace(name, "有限公司", "", -1)
	shorted = strings.Replace(name, "...", "", -1)
	return
}

// 剔除关键词
func RejectWord(str string, word string) string {
	return strings.Replace(str, word, "", -1)
}