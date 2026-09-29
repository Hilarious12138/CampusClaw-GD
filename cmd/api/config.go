package main

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	DBAddr, DBName, DBUser, DBPassword, SessionSecret, Origin, DevOrigin, Listen, UploadDir string
	Secure                                                                                  bool
}

func loadConfig() (Config, error) {
	c := Config{DBAddr: os.Getenv("DB_ADDR"), DBName: os.Getenv("MYSQL_DATABASE"), DBUser: os.Getenv("MYSQL_USER"), DBPassword: os.Getenv("MYSQL_PASSWORD"), SessionSecret: os.Getenv("SESSION_SECRET"), Origin: os.Getenv("APP_ORIGIN"), DevOrigin: os.Getenv("DEV_ORIGIN"), Listen: os.Getenv("LISTEN_ADDR"), UploadDir: os.Getenv("UPLOAD_TMP_DIR"), Secure: os.Getenv("COOKIE_SECURE") == "true"}
	if c.DBAddr == "" {
		c.DBAddr = "db:3306"
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.UploadDir == "" {
		c.UploadDir = "/tmp/campus-uploads"
	}
	if c.DBName == "" || c.DBUser == "" || c.DBPassword == "" || len(c.SessionSecret) < 32 || c.Origin == "" {
		return c, errors.New("缺少数据库、SESSION_SECRET 或 APP_ORIGIN 环境变量")
	}
	for _, s := range []string{c.Origin, c.DevOrigin} {
		if s == "" {
			continue
		}
		u, e := url.Parse(s)
		if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
			return c, errors.New("来源地址无效")
		}
	}
	if strings.HasPrefix(c.Origin, "https:") && !c.Secure {
		return c, errors.New("HTTPS 必须启用 Secure Cookie")
	}
	return c, nil
}
