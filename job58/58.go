package job58

import (
	"58boss/config"
	"58boss/util"
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	"strconv"
	"strings"
	"sync"
)

var (
	ChromeCaps chrome.Capabilities
	cookie         []selenium.Cookie
)

func Run(wg *sync.WaitGroup) {
begin:
	initCap(ChromeCaps)

	ops := []selenium.ServiceOption{}
	service, err := selenium.NewChromeDriverService(config.SeleniumPath, config.Port, ops...)
	if err != nil {
		fmt.Printf("Error starting the ChromeDriver server: %v", err)
	}
	//延迟关闭服务
	defer service.Stop()

	//2.调用浏览器
	//设置浏览器兼容性，我们设置浏览器名称为chrome
	caps := selenium.Capabilities{
		"browserName": "chrome",
	}
	caps.AddChrome(ChromeCaps)
	wd, err := selenium.NewRemote(caps, "http://127.0.0.1:9515/wd/hub")
	if err != nil {
		panic(err)
	}
	//延迟退出chrome
	defer wd.Quit()

	status := crawl(wd)
	if status == "reopen" {
		service.Stop()
		wd.Quit()
		goto begin
	}

	wg.Done()
	wd.Quit()
}

func initCap(ChromeCaps chrome.Capabilities) {
	ChromeCaps = chrome.Capabilities{
		Prefs: map[string]interface{}{ // 禁止加载图片，加快渲染速度
			"profile.managed_default_content_settings.images": 2,
		},
		Path: "",
		Args: []string{
			//"--headless",
			//"--start-maximized",
			"--no-sandbox",
			"--user-agent=" + util.GetRandomUA(),
			"--disable-gpu",
			"--disable-impl-side-painting",
			"--disable-gpu-sandbox",
			"--disable-accelerated-2d-canvas",
			"--disable-accelerated-jpeg-decoding",
			"--test-type=ui",
			"--ignore-certificate-errors",
			"--incognito",
		},
	}
}

// 返回 regain 为重试
func crawl(wd selenium.WebDriver) (status string) {
	var (
		first   = true // 是否首次爬
		maxPage int    // 最大爬行页数
	)
	for {
		// 从关键词开始遍历，如果是第一次访问访问的页数会多点
		for _, keyword := range config.SearKeywords {
			if first {
				maxPage = 10
			} else {
				maxPage = 2
			}

			var trueUrl string
			for i := 1; i <= maxPage; i++ {
				var url string
				if i ==1 {
					url = `https://sz.58.com/job/?key=` + keyword
				} else {
					url = `https://sz.58.com/job/pn` + strconv.Itoa(i) + util.RejectWord(trueUrl, "https://sz.58.com/job")
				}
				logs.Info("crawl:", url)

				//加载网页
				if err := wd.Get(url); err != nil {
					logs.Error(err)

					// 浏览器没有打开
					if strings.Contains(err.Error(), "chrome not reachable") {
						return "reopen"
					}
					continue
				}
				util.RandSleep(15, 25)
				waitCaptcha(wd)

				if i ==  1 {
					trueUrl, _ = wd.CurrentURL()
				}
			}
		}
	}
}

// 58招聘显示验证码
func waitCaptcha(wd selenium.WebDriver) {
	for {
		html, err := wd.PageSource()
		if err != nil {
			return
		}

		if !strings.Contains(html, "请输入验证码") {
			break
		}

		logs.Warning("Needs verify Captcha for 58!!!")
		util.RandSleep(10, 15)
	}
}

// 返回空为页面无异常，返回 lastPage 为最后一页
func checkPage(wd selenium.WebDriver) (status string) {
	// 是否最后一页
	lastPage, _ := wd.FindElement(selenium.ByCSSSelector, "a[class='next disabled']")
	if lastPage != nil {
		return "lastPage"
	}

	// 是否卡在验证码
	loading, _ := wd.FindElement(selenium.ByCSSSelector, "div.boss-loading")
	if loading != nil {
		return "loading"
	}

	return
}
