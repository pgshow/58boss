package boss

import (
	"58boss/config"
	"58boss/sqlite"
	"58boss/util"
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ChromeCaps = new(chrome.Capabilities)
	cookie     []selenium.Cookie
)

func Run(wg *sync.WaitGroup) {
	var first = true // 是否首次爬

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
		"browserName": "chrome1",
	}
	caps.AddChrome(*ChromeCaps)
	//selenium.Proxy{
	//	Type: "manual",
	//	HTTP: "http://" + (streamproxysorted[2] + ":" + streamproxysorted[3] + "@" + streamproxysorted[0] + ":" + streamproxysorted[1]),
	//	//HTTPPort: httpport,
	//}
	//caps.AddProxy(
	//	selenium.Proxy{
	//	Type:          selenium.Manual,
	//	SOCKS:         "http://127.0.0.1:19008",
	//	SOCKSVersion:  5,
	//	SOCKSUsername: "doge",
	//	SOCKSPassword: "doge",
	//})
	wd, err := selenium.NewRemote(caps, "http://127.0.0.1:9515/wd/hub")
	if err != nil {
		panic(err)
	}
	//延迟退出chrome
	defer wd.Quit()

	status, err := crawl(wd, &first)
	if status == "reopen" {
		//service.Stop()
		wd.Quit()
		goto begin
	}

	logs.Error("End scrape, some unknown error:", err)

	wg.Done()
	wd.Quit()
}

// 返回 regain 为重试
func crawl(wd selenium.WebDriver, first *bool) (status string, err error) {
	defer func() {
		if err != nil && strings.Contains(err.Error(), "chrome not reachable") {
			status = "reopen"
		}
		if p := recover(); p != nil {
			errStr := fmt.Sprintf("%v", p)
			if strings.Contains(errStr, "reopen") {
				status = "reopen"
			}
			logs.Error(errStr)
			logs.Error(err)
		}
	}()

	var (
		maxPage     int // 最大爬行页数
		ScrapeTimes = 0 // Cookie/IP爬行量
	)
	for {
		// 从关键词开始遍历，如果是第一次访问访问的页数会多点
		for _, keyword := range config.SearKeywords {
			if *first {
				maxPage = 10
			} else {
				maxPage = 2
			}
			for i := 1; i <= maxPage; i++ {
				urls := `https://www.zhipin.com/c101280600/?query=` + keyword + `&page=` + strconv.Itoa(i)
				logs.Info("Crawl:", urls)
				//加载网页
				if err = wd.Get(urls); err != nil {
					// 浏览器没有打开
					util.NeedThrowErr(err)

					logs.Error(err)
					//wd.GetCookies()

					continue
				}
				util.RandSleep(15, 25)

				lastPageTag := checkPage(wd, &ScrapeTimes) // 检查页面是否正常加载状态

				//we, err := wd.FindElements(selenium.ByCSSSelector, "span.job-name > a")
				we, err := wd.FindElements(selenium.ByCSSSelector, "div.info-primary")
				if err != nil {
					util.NeedThrowErr(err)
					logs.Error("Can't find any jobs in Boss list page", err)
					continue
				}

				// 获取 job 列表
				var jobUrls []string
				for _, item := range we {
					// 公司规模判断
					staffNumTmp, _ := item.FindElement(selenium.ByCSSSelector, "div.info-company > div > p")
					staffNum, _ := staffNumTmp.Text()
					if util.IsOverStaff(staffNum) {
						continue
					}

					titleTmp, _ := item.FindElement(selenium.ByCSSSelector, "span.job-name > a")
					title, _ := titleTmp.GetAttribute("title")
					url, _ := titleTmp.GetAttribute("href")

					// 排除含有目标关键词的 title
					if util.ContainAny(title, config.RejectKeywords) {
						continue
					}

					// 不爬已经检测过的
					if sqlite.SelectUrl(url) {
						logs.Debug("Did lastTime:", url)
						continue
					}

					jobUrls = append(jobUrls, url)
				}
				logs.Info(fmt.Sprintf("Find %d new matched jobs", len(jobUrls)))

				util.RandSleep(15, 25)

				// 爬取每个招聘的信息
				for _, url := range jobUrls {
					// 先爬工作页面
					logs.Info("Job page crawl:", url)
					if err = wd.Get(url); err != nil {
						// 浏览器没有打开
						util.NeedThrowErr(err)

						logs.Error(err)

						util.RandSleep(10, 15)
						continue
					}

					checkPage(wd, &ScrapeTimes)
					//logs.Error("one")
					//if url == "https://www.zhipin.com/job_detail/2da04ac9efebf4301nV60ty1E1tS.html" {
					//	print("it him")
					//}

					var profile = new(util.JobProfile)

					if status, err := extractJobInfo(wd, profile); !status {
						logs.Error("Extract job profile failed: ", err)
						util.RandSleep(15, 25)
						continue
					}

					//logs.Error("two")

					if util.DropWithCondition(profile) {
						// 过滤客户不要的数据
						util.RandSleep(15, 25)
						continue
					}

					util.RandSleep(15, 25)

					// 继续爬公司介绍页
					companyUrl := profile.CompanyUrl
					logs.Info("Company page crawl:", companyUrl)
					if err = wd.Get(companyUrl); err != nil {
						// 浏览器没有打开
						util.NeedThrowErr(err)

						logs.Error(err)

						util.RandSleep(10, 15)
						continue
					}

					checkPage(wd, &ScrapeTimes)

					if status, err := extractCompanyInfo(wd, profile); !status {
						util.NeedThrowErr(err)
						logs.Error("Extract company info failed: ", err)
					}

					util.JobsChan <- *profile
					sqlite.AddUrl(url) // 成功的标记
					util.RandSleep(20, 45)
				}

				// 到达最后一页
				if lastPageTag == "lastPage" {
					break
				}
			}
		}
		*first = false
	}
}

// 返回空为页面无异常，返回 lastPage 为最后一页
func checkPage(wd selenium.WebDriver, ScrapeTimes *int) (status string) {
	// 是否最后一页
	lastPage, _ := wd.FindElement(selenium.ByCSSSelector, "a[class='next disabled']")
	if lastPage != nil {
		return "lastPage"
	}

	// 是否因为 userAgent 卡住
	loading, _ := wd.FindElement(selenium.ByCSSSelector, "div.boss-loading")
	if loading != nil {
		logs.Error("Page stop with loading, reopen soon")
		time.Sleep(3 * time.Second)
		panic("reopen")
	}

	// 是否出现验证码
	waitCaptcha(wd)

	// IP 被封
	ipBlocked(wd)

	// 爬取暂停
	ScrapedPause(ScrapeTimes)

	return
}

func initCap(ChromeCaps *chrome.Capabilities) {
	*ChromeCaps = chrome.Capabilities{
		Prefs: map[string]interface{}{ // 禁止加载图片，加快渲染速度
			"profile.managed_default_content_settings.images": 1,
		},
		Path: "",
		Args: []string{
			//"--headless",
			"--start-maximized",
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
			//"--proxy-server=http://doge:doge@127.0.0.1:19004",
		},
	}
}

// BOSS直聘显示验证码
func waitCaptcha(wd selenium.WebDriver) {
	for {
		html, err := wd.PageSource()
		if err != nil {
			return
		}

		if !strings.Contains(html, "当前IP地址可能存在异常访问行为") {
			break
		}

		logs.Warning("Needs verify Captcha for BOSS!!!")
		util.Alert("Boss")
		util.RandSleep(10, 15)
	}

}

func ipBlocked(wd selenium.WebDriver) {
	for {
		html, err := wd.PageSource()
		if err != nil {
			return
		}

		if !strings.Contains(html, "当前IP存在多次违规访问行为") {
			break
		}

		logs.Warning("IP has been blocked!!!")
		util.Alert("Boss")
		util.RandSleep(100, 160)
	}

}

// 获取job的信息
func extractJobInfo(wd selenium.WebDriver, profile *util.JobProfile) (success bool, err error) {
	defer func() {
		if p := recover(); p != nil {
		}
	}()

	tmpPrimary, err := wd.FindElement(selenium.ByCSSSelector, "div[class='job-primary detail-box']")
	if err != nil {
		util.NeedThrowErr(err)
		return false, err
	}

	/* ---------- 工作信息 ---------- */
	// title
	titleTmp, _ := tmpPrimary.FindElement(selenium.ByCSSSelector, "h1")
	profile.JobTitle, _ = titleTmp.Text()

	// 工资
	salaryTmp, _ := tmpPrimary.FindElement(selenium.ByCSSSelector, "span.salary")
	profile.SalaryLimit, _ = salaryTmp.Text()
	profile.MinSalary, profile.MaxSalary = util.MinAndMax(profile.SalaryLimit)

	// 经验 学历
	expTmp, _ := tmpPrimary.FindElement(selenium.ByCSSSelector, "div.info-primary > p")
	expHtml, _ := expTmp.GetAttribute("innerHTML")
	exps := strings.Split(expHtml, "<em class=\"dolt\"></em>")

	if len(exps) == 3 {
		profile.Experience = exps[1]
		profile.Education = exps[2]
	}

	// 联系人
	tmpDetail, err := wd.FindElement(selenium.ByCSSSelector, "div.job-detail")
	contactTmp, _ := tmpDetail.FindElement(selenium.ByCSSSelector, "h2.name")
	profile.ContactPerson, _ = contactTmp.Text() // 联系人姓名

	positionTmp, _ := tmpDetail.FindElement(selenium.ByCSSSelector, "p.gray")
	positionHtml, _ := positionTmp.GetAttribute("innerHTML")
	positions := strings.Split(positionHtml, "<em class=\"vdot\">·</em>") // 联系人职位

	if len(positions) == 2 {
		profile.ContactPosition = strings.TrimSpace(positions[0])
	}

	// 招人数
	profile.HireNumber = "-"

	// 时间
	profile.PostDate = time.Now().Format("2006-01-02")

	/* ---------- 公司信息 ---------- */
	// 公司简称
	comBasicTmp, _ := wd.FindElement(selenium.ByCSSSelector, "div.sider-company")
	shortNameTmp, _ := comBasicTmp.FindElement(selenium.ByCSSSelector, "div.company-info")
	profile.CompanyShort, _ = shortNameTmp.Text()
	profile.CompanyShort = util.ShortComName(profile.CompanyShort)

	// 公司基本信息
	comBasicPs, _ := comBasicTmp.FindElements(selenium.ByCSSSelector, "p")
	if len(comBasicPs) == 5 {
		profile.CompanyStaff, _ = comBasicPs[2].Text() // 规模
		profile.Realm, _ = comBasicPs[3].Text()        // 行业
		profile.PostDate, _ = comBasicPs[4].Text()     // 更新日期
		profile.PostDate = util.RejectWord(profile.PostDate, "更新于：")
	}

	// 工商信息
	comIncTmp, err := wd.FindElement(selenium.ByCSSSelector, "div.detail-content")

	fullNameTmp, _ := comIncTmp.FindElement(selenium.ByCSSSelector, "div.name")
	profile.ChineseName, _ = fullNameTmp.Text() // 全称

	comIncs, _ := comIncTmp.FindElements(selenium.ByCSSSelector, "div.level-list > li")
	if comIncs != nil && len(comIncs) == 5 {
		profile.LegalEntity, _ = comIncs[0].Text() // 法人
		profile.LegalEntity = util.RejectWord(profile.LegalEntity, "法定代表人：")

		profile.RegisteredCapital, _ = comIncs[1].Text() // 注册资金
		profile.RegisteredCapital = util.RejectWord(profile.RegisteredCapital, "注册资金：")

		profile.FoundingDay, _ = comIncs[2].Text() // 成立日期
		profile.FoundingDay = util.RejectWord(profile.FoundingDay, "成立日期：")
	}

	// 公司page页
	companyUrlTmp, _ := comIncTmp.FindElement(selenium.ByCSSSelector, "a[ka='job-cominfo']")
	profile.CompanyUrl, _ = companyUrlTmp.GetAttribute("href")

	// 地址
	locationTmp, _ := comIncTmp.FindElement(selenium.ByCSSSelector, "div.location-address")
	profile.Location, _ = locationTmp.Text()

	return true, err
}

// 获取公司的信息
func extractCompanyInfo(wd selenium.WebDriver, profile *util.JobProfile) (success bool, err error) {
	tmpPrimary, err := wd.FindElement(selenium.ByCSSSelector, "div[class='job-sec company-business']")
	if err != nil {
		return false, err
	}

	// 经营范围
	fileds, _ := tmpPrimary.FindElements(selenium.ByCSSSelector, "li")
	if fileds != nil && len(fileds) == 8 {
		profile.OperatingItems, _ = fileds[7].GetAttribute("innerHTML")
		profile.OperatingItems = util.RejectWord(profile.OperatingItems, "<span class=\"t\">经营范围：</span>")
	}

	return true, err
}

// cookie/ip 爬取量到达一定时暂停一会
func ScrapedPause(total *int) {
	var (
		pause     = 20
		longPause = 50
		restart   = 100
	)

	*total += 1

	logs.Debug("Scrape time", *total)
	if *total%restart == 0 {
		// 重启
		util.RandSleepMsg(500, 1000, "Boss Reach the max scrape this time, restart after a long pause", 8)
		panic("reopen")
	} else if *total%longPause == 0 {
		// 长休息
		util.RandSleepMsg(360, 500, "Boss Reach the short scrape time, pause for a while", 10)
	} else if *total%pause == 0 {
		// 短暂休息
		util.RandSleepMsg(180, 360, "Boss Reach the short scrape time, pause for a while", 10)
	}
}
