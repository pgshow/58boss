package save

import (
	"58boss/util"
	"encoding/csv"
	"fmt"
	"github.com/astaxie/beego/logs"
	"io"
	"log"
	"os"
	"reflect"
	"sync"
	"time"
)

func Run(wg *sync.WaitGroup) {
	for item := range util.JobsChan {
		save(&item)
	}
	wg.Done()
}

// 按当前日期保存
func save(item *util.JobProfile) {
	path := fmt.Sprintf("result/%s.csv", time.Now().Format("2006-01-02"))
	if !pathExist(path) {
		createNew(path)
	}
	add(item, path)
}

// 新建
func createNew(path string) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		logs.Error("can not create file %s , err: ", path, err)
	}
	defer f.Close()

	f.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(f)
	defer writer.Flush()

	//将爬取信息写入csv文件
	writer.Write([]string{
		"来源",
		"时间",
		"公司规模",
		"英文名字",
		"中文名字",
		"公司简称",
		"法人",
		"成立日期",
		"注册资本(万元）",
		"联系人",
		"联系人职位",
		"职位名称",
		"最低工资",
		"最高工资",
		"工资范围",
		"招人数",
		"工作经验 （年数）",
		"学历要求",
		"行业",
		"公司经营地址",
		"经营范围",
	})
}

// 追加数据
func add(item *util.JobProfile, path string) {
	if !reflect.ValueOf(item).IsValid() {
		return
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("can not open file %s, err is %+v", path, err)
	}
	defer f.Close()
	f.Seek(0, io.SeekEnd)

	w := csv.NewWriter(f)
	//设置属性
	w.UseCRLF = true
	row := struct2strings(item)
	err = w.Write(row)
	if err != nil {
		log.Fatalf("can't write to %s, err is %+v", path, err)
	}
	//这里必须刷新，才能将数据写入文件。
	w.Flush()
}

// 文件或文件夹是否存在
func pathExist(_path string) bool {
	_, err := os.Stat(_path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// struct 转 []string
func struct2strings(item *util.JobProfile) (result []string) {
	v := reflect.ValueOf(*item)
	count := v.NumField()
	for i := 0; i < count; i++ {
		if i == 0 {
			// 第一个 CompanyUrl 不保存进 csv
			continue
		}
		f := v.Field(i)
		result = append(result, f.String())
	}
	return
}
