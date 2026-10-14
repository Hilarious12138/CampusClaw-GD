package main

import (
	"errors"
	"net/url"
	"os"
)

type Config struct {
	DBAddr, DBName, DBUser, DBPassword, JWTSecret, Origin, DevOrigin, Listen, UploadDir string
}

func loadConfig() (Config, error) {
	c := Config{DBAddr: os.Getenv("DB_ADDR"), DBName: os.Getenv("MYSQL_DATABASE"), DBUser: os.Getenv("MYSQL_USER"), DBPassword: os.Getenv("MYSQL_PASSWORD"), JWTSecret: os.Getenv("JWT_SECRET"), Origin: os.Getenv("APP_ORIGIN"), DevOrigin: os.Getenv("DEV_ORIGIN"), Listen: os.Getenv("LISTEN_ADDR"), UploadDir: os.Getenv("UPLOAD_TMP_DIR")}
	if c.DBAddr == "" {
		c.DBAddr = "db:3306"
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.UploadDir == "" {
		c.UploadDir = "/tmp/campus-uploads"
	}
	if c.DBName == "" || c.DBUser == "" || c.DBPassword == "" || len(c.JWTSecret) < 32 || c.Origin == "" {
		return c, errors.New("缺少数据库、JWT_SECRET 或 APP_ORIGIN 环境变量")
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
	return c, nil
}
