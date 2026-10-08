package global

import (
	"im-project/models"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitMySQL() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("未设置 MYSQL_DSN，请先在启动终端配置数据库连接信息，详见 README.md")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("MySQL连接失败，服务器启动终止: %v", err)
	}

	DB = db

	err = DB.AutoMigrate(
		&models.User{},
		&models.Message{},
	)
	if err != nil {
		log.Fatalf("数据库建表失败: %v", err)
	}

	log.Println("MySQL连接成功，数据表检查完成")
}
