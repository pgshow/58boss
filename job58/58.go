package job58

import (
	"58boss/config"
	"58boss/sqlite"
	"58boss/util"
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ChromeCaps = new(chrome.Capabilities)
	cookie     []selenium.Cookie
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
		"browserName": "chrome2",
	}
	caps.AddChrome(*ChromeCaps)
	wd, err := selenium.NewRemote(caps, "http://127.0.0.1:9515/wd/hub")
	if err != nil {
		panic(err)
	}
	//延迟退出chrome
	defer wd.Quit()

	status, _ := crawl(wd)
	if status == "reopen" {
		service.Stop()
		wd.Quit()
		goto begin
	}

	wg.Done()
	wd.Quit()
}

func initCap(ChromeCaps *chrome.Capabilities) {
	*ChromeCaps = chrome.Capabilities{
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
			"--proxy-server=http://127.0.0.1:8080",
		},
	}
}

// 返回 reopen 为重试
func crawl(wd selenium.WebDriver) (status string, err error) {
	defer func() {
		if p := recover(); p != nil {
			errStr := fmt.Sprintf("%v", p)
			if strings.Contains(errStr, "reopen") {
				status = "reopen"
			}
			logs.Error(err)
		}
	}()

	var (
		first   = true // 是否首次爬
		maxPage int    // 最大爬行页数
	)
	for {
		// 从关键词开始遍历，如果是第一次访问访问的页数会多点
		for _, keyword := range config.SearKeywords {
			if err = searchKeywords(wd, keyword); err != nil {
				panic("reopen") // 打开招聘页时遇到任何错误都重开窗口重试
			}

			if first {
				maxPage = 15
			} else {
				maxPage = 5
			}

			for i := 1; i <= maxPage; i++ {
				logs.Info(fmt.Sprintf("crawl %s page %d:", keyword, i))
				if i > 1 {
					// 翻页
					if err = nextPage(wd); err != nil {
						util.NeedThrowErr(err)
					}
				}

				util.RandSleep(15, 25)

				ls, err := wd.FindElements(selenium.ByCSSSelector, "a[tongji_label='listclick']")
				if err != nil {
					util.NeedThrowErr(err)
					logs.Error("can't find any jobs in 58 list page", err)
					continue
				}

				// 获取 job 列表
				var jobObjs []selenium.WebElement
				for _, item := range ls {
					title, _ := item.Text()
					jobId := getJobID(item)

					// 排除含有目标关键词的 title
					if util.ContainAny(title, config.RejectKeywords) {
						continue
					}

					// 查询ID，不爬已经检测过的
					if sqlite.SelectUrl(jobId) {
						logs.Debug("Pass:", jobId)
						continue
					}
					jobObjs = append(jobObjs, item)

				}
				logs.Info(fmt.Sprintf("58同城 find %d new matched jobs", len(jobObjs)))

				util.RandSleep(3, 6)

				// 爬取每个招聘的信息
				for _, jobObj := range jobObjs {
					// 先爬工作页面
					logs.Info("58Job page crawl:", getJobID(jobObj))
					if err = jobObj.Click(); err != nil {
						util.NeedThrowErr(err)
						continue
					}
				}
			}

		}
		first = false
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

// 以获取session的方式搜索
// 返回 reopen 重新打开, 返回 next 下一步, 返回 fail 失败
func searchKeywords(wd selenium.WebDriver, keyword string) (err error) {
	defer func() {
		if p := recover(); p != nil {
			logs.Error(err)
		}
	}()

	//打开首页
	url := "https://sz.58.com/"

	logs.Debug("Open: ", url)

	if err := wd.Get(url); err != nil {
		return err
	}

	util.RandSleep(8, 12)

	// 找到招聘入口
	jobBtn, err := wd.FindElement(selenium.ByCSSSelector, "a[tongji_tag='pc_home_dh_zp']")
	if err != nil {
		return err
	}

	logs.Debug("Open: 58深圳招聘主页")

	// 点击招聘链接
	err = jobBtn.Click()
	if err != nil {
		logs.Error(err)
		return err
	}

	util.RandSleep(4, 5)

	// 关闭首页窗口，切换到招聘窗口
	firstHandle, err := wd.CurrentWindowHandle()
	windows, err := wd.WindowHandles()
	for _, w := range windows {
		if w != firstHandle {
			err := wd.Close()
			if err != nil {
				return err
			}
			err = wd.SwitchWindow(w)
			if err != nil {
				return err
			}
		}
	}

	input, err := wd.FindElement(selenium.ByCSSSelector, "input[id='keyword1']")
	if err != nil {
		return err
	}
	err = input.SendKeys(keyword)
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	submitBtn, err := wd.FindElement(selenium.ByCSSSelector, "a[id='searJob']")
	err = submitBtn.Click()
	if err != nil {
		return err
	}

	return
}

// 翻页
func nextPage(wd selenium.WebDriver) (err error) {
	defer func() {
		if p := recover(); p != nil {
			logs.Error(err)
		}
	}()

	// 找到下一页图标
	next, err := wd.FindElement(selenium.ByCSSSelector, "a.next")
	if err != nil {
		return err
	}
	err = next.Click()
	if err != nil {
		return err
	}
	return
}

// 点击招聘信息页

//func extractJobInfo(wd selenium.WebDriver, profile *util.JobProfile) (success bool, err error) {
//	tmpPrimary, err := wd.FindElement(selenium.ByCSSSelector, "div[class='job-primary detail-box']")
//	if err != nil {
//		return false, err
//	}
//}

// 从url提取job唯一ID
func getJobID(obj selenium.WebElement) (id string) {
	url, err := obj.GetAttribute("href")
	if err != nil {
		return
	}

	tmp := regexp.MustCompile(`entinfo=(\d+?)_[qj]`).FindStringSubmatch(url)
	if tmp == nil {
		return
	}
	return tmp[1]
}
