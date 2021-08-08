package job58

import (
	"58boss/config"
	"58boss/sqlite"
	"58boss/util"
	"errors"
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

	status, err := crawl(wd)
	if status == "reopen" {
		//service.Stop()
		wd.Quit()
		goto begin
	}

	logs.Error("End scrape, some unknown error:", err)

	wg.Done()
	wd.Quit()
}

func initCap(ChromeCaps *chrome.Capabilities) {
	*ChromeCaps = chrome.Capabilities{
		Prefs: map[string]interface{}{ // 禁止加载图片，加快渲染速度
			"profile.managed_default_content_settings.images": 1,
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
			"--incognito",
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
		if err != nil && strings.Contains(err.Error(), "chrome not reachable") {
			status = "reopen"
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
				maxPage = 20
			} else {
				maxPage = 5
			}

			for i := 1; i <= maxPage; i++ {
				logs.Info(fmt.Sprintf("Crawl %s page %d:", keyword, i))
				if i > 1 {
					// 翻页
					if err = nextPage(wd); err != nil {
						util.NeedThrowErr(err)
					}
				}

				util.RandSleep(15, 25)

				waitCaptcha(wd)
				checkResult := checkPage(wd) // 检查页面是否正常加载状态

				ls, err := wd.FindElements(selenium.ByCSSSelector, "li[class='job_item clearfix']")
				if err != nil {
					util.NeedThrowErr(err)
					logs.Error("Can't find any jobs in 58 list page", err)
					continue
				}

				// 获取 job 列表
				var jobObjs []selenium.WebElement
				for _, item := range ls {
					// 丢弃带培训标签的信息
					if itemPeixun, _ := item.FindElement(selenium.ByCSSSelector, "i[class='comp_icons pxdz']"); itemPeixun != nil {
						continue
					}

					item, _ = item.FindElement(selenium.ByCSSSelector, "a[tongji_label='listclick']")

					title, _ := item.Text()
					jobId := getJobID(item)

					// 标题必须包含设置的关键词
					if !util.ContainAny(title, config.SearKeywords) {
						continue
					}

					// 排除含有目标关键词的 title
					if util.ContainAny(title, config.RejectKeywords) {
						continue
					}

					// 查询ID，不爬已经检测过的
					if sqlite.SelectUrl(jobId) {
						logs.Debug("Did lastTime:", jobId)
						continue
					}
					jobObjs = append(jobObjs, item)

				}
				logs.Info(fmt.Sprintf("Find %d new matched jobs", len(jobObjs)))

				util.RandSleep(3, 6)

				// 爬取每个招聘的信息
				for _, jobObj := range jobObjs {
					var processSuccess bool // 标记整个流程成功完成

					// 先爬工作页面
					title, _ := jobObj.Text()
					if title == "外贸业务员" {
						print("now")
					}
					url, _ := jobObj.GetAttribute("href")
					logs.Info("Job page crawl:", url)
					//logs.Info("58Job page crawl:", title, getJobID(jobObj))
					time.Sleep(time.Second)

					// 打开对应工作页
					if err = jobObj.Click(); err != nil {
						util.NeedThrowErr(err)
						continue
					}

					time.Sleep(2 * time.Second)

					listHandler, err := switch2window(wd) // 窗口的handler切换到工作页面
					if err != nil {
						return "", err
					}

					util.RandSleep(15, 25)
					checkPage(wd)

					var profile = new(util.JobProfile)

					// 提取工作信息
					if status, err := extractJobInfo(wd, profile); !status {
						logs.Error("Extract job profile failed: ", err)
						goto jobScrapeOver
					}

					logs.Info("Company page crawl:", profile.CompanyUrl)
					err = openCompany(wd) // 点击打开公司页面
					if err != nil {
						logs.Error("Open company info failed: ", err)
						goto jobScrapeOver
					}

					checkPage(wd)

					if status, err := extractCompanyInfo(wd, profile); !status {
						logs.Error("Extract company info failed: ", err)
						goto jobScrapeOver
					}

					processSuccess = true

				jobScrapeOver:
					time.Sleep(2 * time.Second)
					backListPage(listHandler, wd)

					sqlite.AddUrl(getJobID(jobObj)) // 爬过的标记

					if processSuccess {
						util.JobsChan <- *profile
					}
					util.RandSleep(20, 45)

				}

				// 到达最后一页
				if checkResult == "lastPage" {
					break
				}
			}

		}
		first = false
	}
}

// 切换窗口
func switch2window(wd selenium.WebDriver) (lastHandler string, err error) {
	mainHandler, err := wd.CurrentWindowHandle()
	windows, err := wd.WindowHandles()
	for _, w := range windows {
		if w != mainHandler {
			if err = wd.SwitchWindow(w); err != nil {
				return "", err
			} else {
				return mainHandler, err
			}
		}
	}
	return
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
		util.Alert("58")
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

	// 502错误
	if html, err := wd.PageSource(); err == nil {
		if strings.Contains(html, "502 Bad Gateway") {
			wd.Refresh()
			logs.Debug("502 error, refresh page")
			util.RandSleep(10, 15)
			return "err502"
		}
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

func extractJobInfo(wd selenium.WebDriver, profile *util.JobProfile) (success bool, err error) {
	tmpPrimary, err := wd.FindElement(selenium.ByCSSSelector, "div[class='item_con pos_info']")
	if err != nil {
		util.NeedThrowErr(err)
		return false, err
	}

	/* ---------- 工作信息 ---------- */
	// title
	titleTmp, _ := tmpPrimary.FindElement(selenium.ByCSSSelector, "span.pos_name")
	profile.JobTitle, _ = titleTmp.Text()

	// 招人数, 学历, 经验
	if bases, _ := tmpPrimary.FindElements(selenium.ByCSSSelector, "div.pos_base_condition > span"); len(bases) == 3 {
		profile.HireNumber, _ = bases[0].Text()
		profile.Education, _ = bases[1].Text()
		profile.Experience, _ = bases[2].Text()
	}

	// 地址
	locationTmp, _ := tmpPrimary.FindElement(selenium.ByCSSSelector, "div.pos-area > span:nth-child(2)")
	profile.Location, _ = locationTmp.Text()

	// 工资
	salaryTmp, _ := tmpPrimary.FindElement(selenium.ByCSSSelector, "span.pos_salary")
	profile.SalaryLimit, _ = salaryTmp.Text()

	// 公司主页, 行业, 规模
	comTmp, _ := wd.FindElement(selenium.ByCSSSelector, "div[class='subitem_con company_baseInfo']")

	comUrlTmp, _ := comTmp.FindElement(selenium.ByCSSSelector, "div.baseInfo_link > a")
	profile.CompanyUrl, _ = comUrlTmp.GetAttribute("href")

	comRealmTmp, _ := comTmp.FindElement(selenium.ByCSSSelector, "p.comp_baseInfo_belong")
	profile.Realm, _ = comRealmTmp.Text()

	comStaffTmp, _ := comTmp.FindElement(selenium.ByCSSSelector, "p.comp_baseInfo_scale")
	profile.CompanyStaff, _ = comStaffTmp.Text()

	return true, err
}

// 获取公司的信息
func extractCompanyInfo(wd selenium.WebDriver, profile *util.JobProfile) (success bool, err error) {
	html, err := wd.PageSource()
	if err != nil {
		util.NeedThrowErr(err)
		return false, err
	}

	// 获取Joson段然后提取信息
	if jsonTmp := regexp.MustCompile(`var __REACT_SSR_ =(.+?)\n\s+// 将UID放置到变量中`).FindStringSubmatch(html); jsonTmp != nil {
		jsonCode := jsonTmp[1]

		// 公司全程
		if tmp := regexp.MustCompile(`entName":"(.+?)"`).FindStringSubmatch(jsonCode); tmp != nil {
			profile.ChineseName = tmp[1]
		}

		// 公司简称
		if tmp := regexp.MustCompile(`aliasName":"(.+?)"`).FindStringSubmatch(jsonCode); tmp != nil {
			if !strings.Contains(tmp[1], "\",") {
				profile.CompanyShort = tmp[1]
			}
		}

		// 法人
		if tmp := regexp.MustCompile(`legalPersonName":"(.+?)"`).FindStringSubmatch(jsonCode); tmp != nil {
			profile.LegalEntity = tmp[1]
		}

		// 注册资本
		if tmp := regexp.MustCompile(`regCapital":"(.+?)"`).FindStringSubmatch(jsonCode); tmp != nil {
			profile.RegisteredCapital = tmp[1]
		}

		// 注册日期
		if tmp := regexp.MustCompile(`createTime":"(.+?)"`).FindStringSubmatch(jsonCode); tmp != nil {
			profile.FoundingDay = tmp[1]
		}

		// 经营范围
		if tmp := regexp.MustCompile(`businessScope":"(.+?)"`).FindStringSubmatch(jsonCode); tmp != nil {
			profile.OperatingItems = strings.TrimSpace(tmp[1])
		}

		// 来源
		profile.Source = "58Job"

		// 发布时间
		profile.PostDate = time.Now().Format("2006-01-02")
	}

	return true, err
}

// 从url提取job唯一ID
func getJobID(obj selenium.WebElement) (id string) {
	url, err := obj.GetAttribute("href")
	if err != nil {
		return
	}

	tmp := regexp.MustCompile(`entinfo=(\d+?)_[a-z]&`).FindStringSubmatch(url)
	if tmp == nil {
		return
	}
	return tmp[1]
}

// 点击招聘页上的公司名称打开公司介绍页
func openCompany(wd selenium.WebDriver) (err error) {
	if daizhaoBtn, _ := wd.FindElement(selenium.ByCSSSelector, "div.comp_baseInfo_title > a.baseInfo_daizhao"); daizhaoBtn != nil {
		return errors.New("daizhao")
	}

	if daizhaoBtn, _ := wd.FindElement(selenium.ByCSSSelector, "div.comp_baseInfo_title > a.baseInfo_daipei"); daizhaoBtn != nil {
		return errors.New("daipei")
	}

	companyBtn, err := wd.FindElement(selenium.ByCSSSelector, "div.comp_baseInfo_title > div.baseInfo_link > a")
	if err != nil {
		util.NeedThrowErr(err)
		return err
	}

	wd.ExecuteScript("$('.toolBar').remove()", nil) // 删除侧边栏
	time.Sleep(2 * time.Second)

	// 点击对应公司
	if err = companyBtn.Click(); err != nil {
		util.NeedThrowErr(err)
		return err
	}
	return
}

// 关闭当前页，并切换回招聘列表
func backListPage(listHandler string, wd selenium.WebDriver) {
	var err error
	if err = wd.Close(); err == nil {
		err = wd.SwitchWindow(listHandler)
	}
	if err != nil {
		logs.Error("Switch page to list page failed")
		panic("reopen")
	}
}
