#### **1. randomtest_stdtest**

- 基于 NIST SP800-22 测试随机性。每组 1000000 bit 数据，共 1000 组

进入目录
```
cd randomtest_stdtest
```
配置 randomtest_stdtest_config.ini
```
TestNum = 1000
TestType = "N"
TestLevel = 2
```
运行
```
go run randomtest_stdtest.go
```

---

- 基于 国密 《GM/T 0005-2021》测试随机性。每组 20000 bit 数据，共 1 组

进入目录
```
cd randomtest_stdtest
```
配置 randomtest_stdtest_config.ini
```
TestNum = 1000
TestType = "G"
TestLevel = 1
```
运行
```
go run randomtest_stdtest.go
```

---

- 基于 国密 《GM/T 0005-2021》测试随机性。每组 1000000 bit 数据，共 1000 组

进入目录
```
cd randomtest_stdtest
```
配置 randomtest_stdtest_config.ini
```
TestNum = 1000
TestType = "G"
TestLevel = 2
```
运行
```
go run randomtest_stdtest.go
```

---

- 基于 国密 《GM/T 0005-2021》测试随机性。每组 100000000 bit 数据，共 1000 组

进入目录
```
cd randomtest_stdtest
```
配置 randomtest_stdtest_config.ini
```
TestNum = 1000
TestType = "G"
TestLevel = 3
```
运行
```
go run randomtest_stdtest.go
```

#### **2. randomtest_stdtest_vue3_wailsv2**

进入目录
```
cd randomtest_stdtest_vue3_wailsv2
```
运行
```
wails dev
```

#### **3. randomtest_web_gin 服务启动**

- 基于 Gin 的随机性测试 Web 服务

进入目录
```
cd randomtest_web_gin
```
配置 ginserver_config.ini
```
[app]
RunMode = "debug" ; "release"

[server]
Port = ":8888"
ReadTimeout = 60
WriteTimeout = 60
```
运行
```
go run main.go
```
