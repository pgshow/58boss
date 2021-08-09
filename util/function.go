package util

import (
	"bufio"
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/dbatbold/beep"
	"github.com/eddycjy/fake-useragent"
	"io/ioutil"
	"log"
	"math/rand"
	"regexp"
	"strconv"
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
		if s == "" || strings.HasPrefix(s, "#") {
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
	s = rand.Int63n(max-min) + min
	s = (s * 4) / 5

	logs.Info(fmt.Sprintf("wait for %d seconds", s))
	return time.Duration(s) * time.Second
}

// RandSleep 随机延迟
func RandSleep(min, max int64) {
	time.Sleep(randSecond(min, max))
}

// RandSleep 随机延迟带提示
func RandSleepMsg(min, max int, msg string, interval int) {
	var seconds int
	if min >= max || min == 0 || max == 0 {
		seconds = max
	} else {
		seconds = rand.Intn(max-min) + min
	}

	for i := 0; i < seconds; i++ {
		if i%interval == 0 {
			logs.Debug(msg)
		}
		time.Sleep(time.Second)
	}
}

// ShortComName 公司名称简化
func ShortComName(name string) (shorted string) {
	shorted = strings.Replace(name, "深圳市", "", -1)
	shorted = strings.Replace(shorted, "有限公司", "", -1)
	shorted = strings.Replace(shorted, "...", "", -1)
	return shorted
}

// 剔除关键词
func RejectWord(str string, word string) string {
	return strings.Replace(str, word, "", -1)
}

// 是否抛出浏览器错误
func NeedThrowErr(err error) {
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "chrome not reachable") {
		panic("reopen")
	}
	if strings.Contains(err.Error(), "invalid session id") {
		panic("reopen")
	}
}

// 声音提醒
func Alert(which string) {
	var output string

	music := beep.NewMusic(output)
	volume := 100

	//if len(output) > 0 {
	//	fmt.Println("Writing WAV to", output)
	//} else {
	//	beep.PrintSheet = true
	//}

	if err := beep.OpenSoundDevice("default"); err != nil {
		log.Fatal(err)
	}
	if err := beep.InitSoundDevice(); err != nil {
		log.Fatal(err)
	}
	defer beep.CloseSoundDevice()

	musicScore := `
        VP SA8 SR9
        A9HRDE cc DScszs|DEc DQzDE[|cc DScszs|DEc DQz DE[|vv DSvcsc|DEvs ]v|cc DScszs|VN
        A3HLDE [n z,    |cHRq HLz, |[n z,    |cHRq HLz,  |sl z,    |]m   pb|z, ]m    |
        
        A9HRDE cz [c|ss DSsz]z|DEs] ps|DSsz][ z][p|DEpDQ[ [|VN
        A3HLDE [n ov|]m [n    |  pb ic|  n,   lHRq|HLnc DQ[|
    `

	reader := bufio.NewReader(strings.NewReader(musicScore))
	go music.Play(reader, volume)
	music.Wait()
	beep.FlushSoundBuffer()
}

// 计算最低和最高工资
func MinAndMax(str string) (low string, high string) {
	if match := regexp.MustCompile(`(\d+)-(\d+)([K元])`).FindStringSubmatch(str); match != nil {
		lowTmp := match[1]
		highTmp := match[2]

		if unit := match[3]; unit == "K" {
			lowInt, _ := strconv.Atoi(lowTmp)
			highInt, _ := strconv.Atoi(highTmp)
			low = strconv.Itoa(lowInt * 1000)
			high = strconv.Itoa(highInt * 1000)
		} else if unit == "元" {
			low = lowTmp
			high = highTmp
		}
	}

	return
}

// 丢弃不满足定义条件的任务
func DropWithCondition(profile *JobProfile) (drop bool) {
	/* 丢弃公司规模100以上的 */
	if IsOverStaff(profile.CompanyStaff) {
		return true
	}
	return false
}

// 公司规模不能太大
func IsOverStaff(str string) (status bool) {
	/* 丢弃公司规模100以上的 */
	number := 100
	if match := regexp.MustCompile(`(\d+)`).FindAllString(str, 2); match != nil {
		if len(match) == 1 {
			if staff, _ := strconv.Atoi(match[0]); staff > number {
				return true
			}
		}
		if len(match) == 2 {
			staffMin, _ := strconv.Atoi(match[0])
			staffMax, _ := strconv.Atoi(match[1])
			if staffMin < number {
				if staffMax > number {
					return true
				}
			} else if staffMin > number {
				return true
			}
		}
	}
	return false
}
