package seeds

import (
	"os"
	"time"

	"github.com/azumads/faker"

	"github.com/dfang/qor-demo/config/db"
)

var Fake *faker.Faker

var (
	Root, _ = os.Getwd()
	// DraftDB = db.DB
)

var Seeds = struct {
	Categories []struct {
		Name string
		Code string
	}
	Collections []struct {
		Name string
	}
	Colors []struct {
		Name string
		Code string
	}
	Sizes []struct {
		Name string
		Code string
	}
	Materials []struct {
		Name string
		Code string
	}
	Products []struct {
		CategoryName string
		Collections  []struct {
			Name string
		}
		Name          string
		ZhName        string
		NameWithSlug  string
		Code          string
		Price         float32
		Description   string
		ZhDescription string
		MadeCountry   string
		Gender        string
		ZhGender      string
		ZhMadeCountry string

		ColorVariations []struct {
			ColorName string
			ColorCode string
			Images    []struct {
				URL string
			}
		}
		SizeVariations []struct {
			SizeName string
		}
	}
	Stores []struct {
		Name      string
		Phone     string
		Email     string
		Country   string
		Zip       string
		City      string
		Region    string
		Address   string
		Latitude  float64
		Longitude float64
	}
	Setting struct {
		ShippingFee     uint
		GiftWrappingFee uint
		CODFee          uint `gorm:"column:cod_fee"`
		TaxRate         int
		Address         string
		City            string
		Region          string
		Country         string
		Zip             string
		Latitude        float64
		Longitude       float64
	}
	Seo struct {
		SiteName    string
		DefaultPage struct {
			Title       string
			Description string
			Keywords    string
		}
		HomePage struct {
			Title       string
			Description string
			Keywords    string
		}
		ProductPage struct {
			Title       string
			Description string
			Keywords    string
		}
	}
	Enterprises []struct {
		Name           string
		Begins         string
		Expires        string
		RequiresCoupon bool
		Unique         bool

		Coupons []struct {
			Code string
		}
		Rules []struct {
			Kind  string
			Value string
		}
		Benefits []struct {
			Kind  string
			Value string
		}
	}
	Slides []struct {
		Title    string
		SubTitle string
		Button   string
		Link     string
		Image    string
	}
	MediaLibraries []struct {
		Title string
		Image string
	}
	BannerEditorSettings []struct {
		ID    string
		Kind  string
		Value string
	}
	Users []struct {
		Name   string `form:"name"`
		Gender string
		Role   string
		// 身份证号码
		IdentityCardNum string
		// 手机号码
		MobilePhone string
		// 车牌号码
		CarLicencePlateNum string
		// 车型 东风小货
		CarType string
		// 驾照类型 C1
		CarLicenseType string
		// 是否临时工
		IsCasual bool
		// UserType DeliveryMan、SetupMan
		Type      string
		JDAppUser string
		HireDate  *time.Time
	}
}{}

func init() {
	// db.DB.Set(publish2.VisibleMode, publish2.ModeOff).Set(publish2.ScheduleMode, publish2.ModeOff)

	// Fake, _ = faker.New("en")
	// Fake.Rand = rand.New(rand.NewSource(42))
	// rand.Seed(time.Now().UnixNano())

	// filepaths, _ := filepath.Glob(filepath.Join("config", "db", "seeds", "data", "*.yml"))
	// if err := configor.Load(&Seeds, filepaths...); err != nil {
	// 	panic(err)
	// }
}

// TruncateTables Truncate tables
func TruncateTables(tables ...interface{}) {
	for _, table := range tables {
		if err := db.DB.DropTableIfExists(table).Error; err != nil {
			panic(err)
		}

		db.DB.AutoMigrate(table)
	}
}
