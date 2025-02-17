package orders

import (
	"fmt"
	"strings"
	"time"

	// "net/http"

	"github.com/dfang/qor-demo/config/db"
	"github.com/dfang/qor-demo/models/orders"
	"github.com/dfang/qor-demo/models/users"
	"github.com/dfang/qor-demo/utils/funcmapmaker"
	"github.com/jinzhu/gorm"
	"github.com/jinzhu/now"
	"github.com/qor/activity"
	"github.com/qor/admin"
	"github.com/qor/application"
	"github.com/qor/exchange"
	"github.com/qor/qor"
	"github.com/qor/render"
	"github.com/qor/transition"
)

// New new home app
func New(config *Config) *App {
	return &App{Config: config}
}

// NewWithDefault new home app
func NewWithDefault() *App {
	return &App{Config: &Config{}}
}

// App home app
type App struct {
	Config *Config
}

// Config home config struct
type Config struct {
}

var Genders = []string{"Men", "Women", "Kids"}

// ConfigureApplication configure application
func (app App) ConfigureApplication(application *application.Application) {
	controller := &Controller{View: render.New(&render.Config{AssetFileSystem: application.AssetFS.NameSpace("orders")}, "app/orders/views")}

	funcmapmaker.AddFuncMapMaker(controller.View)

	// 订单后台
	app.ConfigureAdmin(application.Admin)

	// 订单回访后台
	app.ConfigureOrderFollowUpsAdmin(application.Admin)

	application.Router.Get("/cart", controller.Cart)
	application.Router.Put("/cart", controller.UpdateCart)
	application.Router.Post("/cart", controller.UpdateCart)
	application.Router.Get("/cart/checkout", controller.Checkout)
	application.Router.Put("/cart/checkout", controller.Checkout)
	application.Router.Post("/cart/complete", controller.Complete)
	application.Router.Post("/cart/complete/creditcard", controller.CompleteCreditCard)
	application.Router.Get("/cart/success", controller.CheckoutSuccess)
	// application.Router.Post("/order/callback/amazon", controller.AmazonCallback)
}

// ConfigureAdmin configure admin interface
func (App) ConfigureAdmin(Admin *admin.Admin) {
	// Add Order
	order := Admin.AddResource(&orders.Order{}, &admin.Config{Menu: []string{"Order Management"}})

	Admin.AddResource(&orders.Rating{}, &admin.Config{Menu: []string{"Order Management"}})

	oi := Admin.AddResource(&orders.OrderItem{}, &admin.Config{Menu: []string{"Order Management"}})
	oi.IndexAttrs("Order", "ItemName", "Range", "Category", "Dimension", "DeliveryFee")
	// oi.IndexAttrs("-SizeVariation", "-ColorVariation", "-Price", "-DiscountRate", "-ProductNo", "-Install")
	oi.ShowAttrs("-SizeVariation", "-ColorVariation", "-Price", "-DiscountRate", "-ProductNo", "-Install")
	oi.EditAttrs("-SizeVariation", "-ColorVariation", "-Price", "-DiscountRate", "-ProductNo", "-Install")
	configureScopesForOrderItems(oi)

	rule := Admin.AddResource(&orders.Rule{}, &admin.Config{Menu: []string{"Order Management"}})
	rule.IndexAttrs("-Conditions")
	// rule.Meta(&admin.Meta{Name: "Conditions", Type: "rule_conditions_field", FormattedValuer: func(record interface{}, _ *qor.Context) (result interface{}) {
	// 	m := record.(*orders.Rule)
	// 	return m
	// }})
	configureScopesForRules(rule)

	Admin.AddResource(&orders.Condition{}, &admin.Config{Menu: []string{"Order Management"}})
	Admin.AddResource(&orders.Execution{}, &admin.Config{Menu: []string{"Order Management"}})
	pricing := Admin.AddResource(&orders.Pricing{}, &admin.Config{Menu: []string{"Order Management"}})
	pricing.Meta(&admin.Meta{
		Name:  "Category",
		Type:  "select_one",
		Label: "分类",
		Config: &admin.SelectOneConfig{
			Collection: []string{"冰箱", "洗衣机", "空调", "彩电", "电脑", "小家电", "厨卫"},
			AllowBlank: false,
		}})
	pricing.Meta(&admin.Meta{
		Name:  "VolumeType",
		Type:  "select_one",
		Label: "件型大小",
		Config: &admin.SelectOneConfig{
			Collection: []string{"超小件", "超小件", "小件", "中件", "大件", "超大件"},
			AllowBlank: false,
		}})
	pricing.Meta(&admin.Meta{
		Name:  "DeliveryArea",
		Type:  "select_one",
		Label: "配送范围",
		Config: &admin.SelectOneConfig{
			Collection: []string{"县城", "乡下"},
			AllowBlank: false,
		}})
	rule.Meta(&admin.Meta{
		Name:  "Category",
		Type:  "select_one",
		Label: "分类",
		Config: &admin.SelectOneConfig{
			Collection: []string{"", "电脑", "电视", "冰箱", "空调", "洗衣机"},
			AllowBlank: true,
		}})
	rule.Meta(&admin.Meta{
		Name:  "Effect",
		Type:  "select_one",
		Label: "作用",
		Config: &admin.SelectOneConfig{
			Collection: []string{"判断分类", "判断大小", "判断配送范围", "定价"},
			AllowBlank: false,
		}})

	condition1 := rule.Meta(&admin.Meta{Name: "Conditions"}).Resource
	condition1.IndexAttrs("-Rule")
	condition1.EditAttrs("-Rule")
	condition1.ShowAttrs("-Rule")
	condition1.NewAttrs("-Rule")
	condition1.Meta(&admin.Meta{
		Name:  "Operator",
		Type:  "select_one",
		Label: "操作符",
		Config: &admin.SelectOneConfig{
			Collection: []string{">", ">=", "=", "<", "<=", "包含", "不包含"},
			AllowBlank: false,
		}})

	execution1 := rule.Meta(&admin.Meta{Name: "Executions"}).Resource
	execution1.IndexAttrs("-Rule")
	execution1.EditAttrs("-Rule")
	execution1.ShowAttrs("-Rule")
	execution1.NewAttrs("-Rule")
	execution1.Meta(&admin.Meta{
		Name:  "Name",
		Type:  "select_one",
		Label: "操作",
		Config: &admin.SelectOneConfig{
			Collection: []string{"设置配送范围", "设置分类", "设置大小", "设置配送价"},
			AllowBlank: false,
		}})

	configureMetas(order)
	configureScopes(order)
	configureActions(Admin, order)

	// Customize visible fields
	// https://doc.getqor.com/admin/fields.html
	configureVisibleFields(order)

	// https://doc.getqor.com/admin/metas/select-one.html
	// Generate options by data from the database
	order.Meta(&admin.Meta{
		Name:  "ManToSetup",
		Type:  "select_one",
		Label: "Man To Setup",
		Config: &admin.SelectOneConfig{
			Collection: func(_ interface{}, context *admin.Context) (options [][]string) {
				var users []users.User
				context.GetDB().Where("role = ?", "setup_man").Find(&users)
				for _, n := range users {
					idStr := fmt.Sprintf("%d", n.ID)
					var option = []string{idStr, n.Name}
					options = append(options, option)
				}
				return options
			},
			AllowBlank: true,
		}})

	order.Meta(&admin.Meta{
		Name: "ManToDeliver",
		Type: "select_one",
		Config: &admin.SelectOneConfig{
			Collection: func(_ interface{}, context *admin.Context) (options [][]string) {
				var users []users.User
				context.GetDB().Where("role = ?", "delivery_man").Find(&users)

				for _, n := range users {
					idStr := fmt.Sprintf("%d", n.ID)
					var option = []string{idStr, n.Name}
					options = append(options, option)
				}
				return options
			},
			AllowBlank: true,
		}})

	// oldSearchHandler := order.SearchHandler
	// order.SearchHandler = func(keyword string, context *qor.Context) *gorm.DB {
	// 	return oldSearchHandler(keyword, context).Where("state <> ? AND state <> ?", "", orders.DraftState)
	// }

	// Add activity for order
	activity.Register(order)

	// 废弃订单
	configureAbandonedOrders(Admin)

	// Delivery Methods
	Admin.AddResource(&orders.DeliveryMethod{}, &admin.Config{Menu: []string{"Site Management"}})

	// installs := Admin.AddResource(&orders.Order{Source: "JD"}, &admin.Config{Name: "Installs", Menu: []string{"Order Management"}})
	// installs.Scope(&admin.Scope{
	// 	Default: true,
	// 	Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
	// 		return db.Where("source IS NOT NULL")
	// 	},
	// })
	// // installs.IndexAttrs("ID", "source", "order_no", "customer_name", "customer_address", "customer_phone", "is_delivery_and_setup", "reserved_delivery_time", "reserved_setup_time", "man_to_deliver_id", "man_to_setup_id", "man_to_pickup_id", "state")

	// installs.IndexAttrs("ID", "source", "order_no", "customer_name", "customer_address", "customer_phone", "is_delivery_and_setup", "reserved_delivery_time", "reserved_setup_time", "man_to_deliver_id", "man_to_setup_id", "man_to_pickup_id", "state")

	// // define scopes for Order
	// for _, state := range []string{"pending", "processing", "delivery_scheduled", "setup_scheduled", "pickup_scheduled", "cancelled", "shipped", "paid_cancelled", "returned"} {
	// 	var state = state
	// 	installs.Scope(&admin.Scope{
	// 		Name:  state,
	// 		Label: strings.Title(strings.Replace(state, "_", " ", -1)),
	// 		Group: "Order Status",
	// 		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
	// 			return db.Where(orders.Order{Transition: transition.Transition{State: strings.Title(state)}})
	// 		},
	// 	})
	// }

	// Define Resource
	o1 := exchange.NewResource(&orders.Order{}, exchange.Config{PrimaryField: "customer_name"})
	// Define columns are exportable/importable
	o1.Meta(&exchange.Meta{Name: "customer_name"})
	o1.Meta(&exchange.Meta{Name: "customer_address"})
	o1.Meta(&exchange.Meta{Name: "customer_phone"})
	o1.Meta(&exchange.Meta{Name: "receivables"})
	o1.Meta(&exchange.Meta{Name: "reserved_delivery_time"})
	o1.Meta(&exchange.Meta{Name: "reserved_setup_time"})

}

func sizeVariationCollection(resource interface{}, context *qor.Context) (results [][]string) {
	// for _, sizeVariation := range products.SizeVariations() {
	// 	results = append(results, []string{strconv.Itoa(int(sizeVariation.ID)), sizeVariation.Stringify()})
	// }
	return
}

func configureVisibleFields(order *admin.Resource) {
	// order.IndexAttrs("ID", "User", "PaymentAmount", "ShippedAt", "CancelledAt", "State", "ShippingAddress")
	// order.IndexAttrs("ID", "source", "order_no", "state", "order_type", "customer_name", "customer_address", "customer_phone", "receivables", "is_delivery_and_setup", "reserved_delivery_time", "reserved_setup_time", "man_to_deliver_id", "man_to_setup_id", "man_to_pickup_id", "shipping_fee", "setup_fee", "pickup_fee")
	// order.Meta(&admin.Meta{Name: "操作", Valuer: func(record interface{}, context *qor.Context) interface{} {
	// 	if _, ok := record.(*orders.Order); ok {
	// 		// return "<a href='#'>View</a>"
	// 		return "xxx"
	// 	}
	// 	return ""
	// }})

	// order.IndexAttrs("ID", "source", "order_no", "state", "order_type", "customer_name", "customer_address", "customer_phone", "receivables",
	// 	"is_delivery_and_setup", "reserved_delivery_time", "reserved_setup_time", "man_to_deliver_id", "man_to_setup_id", "man_to_pickup_id",
	//   "shipping_fee", "setup_fee", "pickup_fee", "created_at", "updated_at")

	order.IndexAttrs("ID", "source", "order_no", "customer_name", "customer_address", "customer_phone", "receivables",
		"is_delivery_and_setup", "reserved_delivery_time", "reserved_setup_time", "created_at", "updated_at")

	// orderItems := order.GetAdmin().NewResource(&orders.OrderItem{})
	// orderItems.IndexAttrs("Price", "Quantity")
	// orderItems.ShowAttrs("Price", "Quantity")

	a1 := []string{"-CreatedBy", "-UpdatedBy", "-User", "-DeliveryMethod", "-PaymentMethod", "-TrackingNumber", "-ShippedAt", "-ReturnedAt", "-CancelledAt", "-ShippingAddress", "-BillingAddress", "-IsDeliveryAndSetup"}
	a2 := []string{"-DiscountValue", "-AbandonedReason", "-PaymentLog", "-PaymentAmount", "-PaymentTotal", "-AmazonOrderReferenceID", "-AmazonAddressAccessToken"}
	a3 := append(append([]string{}, a1...), a2...)

	order.Meta(&admin.Meta{
		Name: "OrderItems",
		Type: "collection_edit",
	})

	items := order.Meta(&admin.Meta{Name: "OrderItems"}).Resource
	items.IndexAttrs("OrderNo", "ItemName", "Quantity")
	items.ShowAttrs("OrderNo", "ItemName", "Quantity")
	items.EditAttrs("OrderNo", "ItemName", "Quantity")

	order.NewAttrs(a3, "-CreatedAt", "-UpdatedAt")

	order.EditAttrs(a3, "-CreatedAt", "-UpdatedAt")

	order.ShowAttrs(a3)

	order.SearchAttrs("customer_name", "customer_phone", "order_no")
}

func configureMetas(order *admin.Resource) {
	// orderItemMeta := order.Meta(&admin.Meta{Name: "OrderItems"})
	// orderItemMeta.Resource.Meta(&admin.Meta{Name: "SizeVariation", Config: &admin.SelectOneConfig{Collection: sizeVariationCollection}})
	// order.NewAttrs("CustomerName", "CustomerAddress", "CustomerPhone", "Source", "OrderType", "Receivables", "PickupFee", "ShippingFee", "SetupFee")
	order.Meta(&admin.Meta{Name: "ReservedDeliveryTime", Type: "date"})
	order.Meta(&admin.Meta{Name: "ReservedSetupTime", Type: "date"})
	order.Meta(&admin.Meta{Name: "ReservedPickupTime", Type: "date"})

	/*
	   // order.Meta(&admin.Meta{Name: "ShippedAt", Type: "date"})
	   // order.Meta(&admin.Meta{Name: "ReturnedAt", Type: "date"})
	   // order.Meta(&admin.Meta{Name: "CancelledAt", Type: "date"})

	   // order.Meta(&admin.Meta{Name: "ShippingAddress", Type: "single_edit"})
	   // order.Meta(&admin.Meta{Name: "BillingAddress", Type: "single_edit"})

	   // order.Meta(&admin.Meta{Name: "PaymentLog", Type: "readonly", FormattedValuer: func(record interface{}, _ *qor.Context) (result interface{}) {
	   // 	order := record.(*orders.Order)
	   // 	return template.HTML(strings.Replace(strings.TrimSpace(order.PaymentLog), "\n", "<br>", -1))
	   // }})

	   // order.Meta(&admin.Meta{Name: "DeliveryMethod", Type: "select_one",
	   // 	Config: &admin.SelectOneConfig{
	   // 		Collection: func(_ interface{}, context *admin.Context) (options [][]string) {
	   // 			var methods []orders.DeliveryMethod
	   // 			context.GetDB().Find(&methods)
	   // 			for _, m := range methods {
	   // 				idStr := fmt.Sprintf("%d", m.ID)
	   // 				var option = []string{idStr, fmt.Sprintf("%s (%0.2f) руб", m.Name, m.Price)}
	   // 				options = append(options, option)
	   // 			}
	   // 			return options
	   // 		},
	   // 	},
	   // })
	*/
	order.Meta(&admin.Meta{Name: "Source", Type: "select_one", Config: &admin.SelectOneConfig{Collection: orders.SOURCES}})
	order.Meta(&admin.Meta{Name: "OrderType", Type: "select_one", Config: &admin.SelectOneConfig{Collection: orders.ORDER_TYPES}})
	// order.Meta(&admin.Meta{Name: "CreatedAt", Type: "datetime", FormattedValuer: func(record interface{}, _ *qor.Context) (result interface{}) {
	// 	order := record.(*orders.Order)
	// 	return order.CreatedAt.Local().Format("2006-01-02 15:04:05")
	// }})
	// order.Meta(&admin.Meta{Name: "UpdatedAt", Type: "date", FormattedValuer: func(record interface{}, _ *qor.Context) (result interface{}) {
	// 	order := record.(*orders.Order)
	// 	return order.UpdatedAt.Local().Format("2006-01-02 15:04:05")
	// }})
	// order.Meta(&admin.Meta{Name: "customer_address", Type: "string", FormattedValuer: func(record interface{}, _ *qor.Context) (result interface{}) {
	// 	order := record.(*orders.Order)
	// 	return strings.Replace(order.CustomerAddress, "江西九江市修水县", "", -1)
	// }})

	// order.Meta(&admin.Meta{Name: "customer_phone", Type: "string", FormattedValuer: func(record interface{}, _ *qor.Context) (result interface{}) {
	// 	order := record.(*orders.Order)
	// 	phones := strings.Split(order.CustomerPhone, "/")
	// 	if len(phones) > 1 && phones[0] == phones[1] {
	// 		return phones[0]
	// 	}
	// 	return order.CustomerPhone
	// }})

	// order.Meta(&admin.Meta{Name: "man_to_deliver_id", Type: "string", FormattedValuer: func(record interface{}, ctx *qor.Context) (result interface{}) {
	// 	order := record.(*orders.Order)
	// 	if order.ManToDeliverID != "" {
	// 		var user users.User
	// 		ctx.DB.Where("id = ?", order.ManToDeliverID).Find(&user)
	// 		return user.Name
	// 	}
	// 	return ""
	// }})
	// order.Meta(&admin.Meta{Name: "man_to_setup_id", Type: "string", FormattedValuer: func(record interface{}, ctx *qor.Context) (result interface{}) {
	// 	order := record.(*orders.Order)
	// 	if order.ManToSetupID != "" {
	// 		var user users.User
	// 		ctx.DB.Where("id = ?", order.ManToSetupID).Find(&user)
	// 		return user.Name
	// 	}
	// 	return ""
	// }})
	// order.Meta(&admin.Meta{Name: "man_to_pickup_id", Type: "string", FormattedValuer: func(record interface{}, ctx *qor.Context) (result interface{}) {
	// 	order := record.(*orders.Order)
	// 	if order.ManToPickUpID != "" {
	// 		var user users.User
	// 		ctx.DB.Where("id = ?", order.ManToPickUpID).Find(&user)
	// 		return user.Name
	// 	}
	// 	return ""
	// }})
}

func configureScopes(order *admin.Resource) {
	// define scopes for Order
	order.Scope(&admin.Scope{
		Name:  "ReservedToday",
		Label: "Today",
		Group: "Filter By Date",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("created_at >= ?", now.BeginningOfDay()).Where("created_at <=? ", time.Now())
		},
	})
	order.Scope(&admin.Scope{
		Name:  "ReservedYesterday",
		Label: "Yesterday",
		Group: "Filter By Date",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			now.WeekStartDay = time.Monday
			// select order_no, customer_name, item_name::varchar(20), quantity, created_at
			// from orders_view
			// where created_at between now() - interval '2 day' and  now() - interval '1 day';
			// return db.Where("created_at between now() - interval '2 day' and  now() - interval '1 day'")
			return db.Where("created_at >= ?", now.BeginningOfDay().AddDate(0, 0, -1)).Where("created_at <=? ", now.EndOfDay().AddDate(0, 0, -1))
		},
	})
	order.Scope(&admin.Scope{
		Name:  "The Day Before Yesterday",
		Label: "The Day Before Yesterday",
		Group: "Filter By Date",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			now.WeekStartDay = time.Monday
			// select order_no, customer_name, item_name::varchar(20), quantity, created_at
			// from orders_view
			// where created_at between now() - interval '2 day' and  now() - interval '1 day';
			// return db.Where("created_at between now() - interval '2 day' and  now() - interval '1 day'")
			return db.Where("created_at >= ?", now.BeginningOfDay().AddDate(0, 0, -2)).Where("created_at <=? ", now.EndOfDay().AddDate(0, 0, -2))
		},
	})

	order.Scope(&admin.Scope{
		Name:  "Last3Days",
		Label: "三天前",
		Group: "Filter By Date",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("created_at >= ?", now.BeginningOfDay().AddDate(0, 0, -3)).Where("created_at <=? ", now.EndOfDay().AddDate(0, 0, -3))
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ThisWeek",
		Label: "This Week",
		Group: "Filter By Date",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			now.WeekStartDay = time.Monday
			return db.Where("created_at >= ?", now.BeginningOfWeek()).Where("created_at <=? ", now.EndOfWeek())
		},
	})
	order.Scope(&admin.Scope{
		Name:  "ThisMonth",
		Label: "This Month",
		Group: "Filter By Date",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			now.WeekStartDay = time.Monday
			return db.Where("created_at >= ?", now.BeginningOfMonth()).Where("created_at <=? ", now.EndOfMonth())
		},
	})
	order.Scope(&admin.Scope{
		Name:  "ThisQuarter",
		Label: "This Quarter",
		Group: "Filter By Date",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("created_at >= ?", now.BeginningOfQuarter()).Where("created_at <=? ", now.EndOfQuarter())
		},
	})
	order.Scope(&admin.Scope{
		Name:  "ThisYear",
		Label: "This Year",
		Group: "Filter By Date",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("created_at >= ?", now.BeginningOfYear()).Where("created_at <=? ", now.EndOfYear())
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToDeliverToday",
		Label: "ToDeliverToday",
		Group: "Filter By DeliveryDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_delivery_time = ?", now.BeginningOfDay().Format("2006-01-02"))
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToDeliverTomorrow",
		Label: "ToDeliverTomorrow",
		Group: "Filter By DeliveryDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_delivery_time = ?", now.BeginningOfDay().AddDate(0, 0, 1).Format("2006-01-02"))
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToDeliverTomorrow2",
		Label: "ToDeliverTomorrow2",
		Group: "Filter By DeliveryDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_delivery_time = ?", now.BeginningOfDay().AddDate(0, 0, 2).Format("2006-01-02"))
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToSetupToday",
		Label: "ToSetupToday",
		Group: "Filter By SetupDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_setup_time = ?", now.BeginningOfDay().Format("2006-01-02"))
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToSetupTomorrow",
		Label: "ToSetupTomorrow",
		Group: "Filter By SetupDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_setup_time = ?", now.BeginningOfDay().AddDate(0, 0, 1).Format("2006-01-02"))
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToSetupTomorrow2",
		Label: "ToSetupTomorrow2",
		Group: "Filter By SetupDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_setup_time = ?", now.BeginningOfDay().AddDate(0, 0, 2).Format("2006-01-02"))
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToPickUpToday",
		Label: "ToPickUpToday",
		Group: "Filter By PickupDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_delivery_time = ?", now.BeginningOfDay().Format("2006-01-02")).Where("order_no like ?", "Q%")
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToPickUpTomorrow",
		Label: "ToPickUpTomorrow",
		Group: "Filter By PickupDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_delivery_time = ?", now.BeginningOfDay().AddDate(0, 0, 1).Format("2006-01-02")).Where("order_no like ?", "Q%")
		},
	})

	order.Scope(&admin.Scope{
		Name:  "ToPickUpTomorrow2",
		Label: "ToPickUpTomorrow2",
		Group: "Filter By PickupDate",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("reserved_delivery_time = ?", now.BeginningOfDay().AddDate(0, 0, 2).Format("2006-01-02")).Where("order_no like ?", "Q%")
		},
	})

	order.Filter(&admin.Filter{
		Name: "created_at",
		Config: &admin.DatetimeConfig{
			ShowTime: false,
		},
	})

	// filter by order state
	// for _, state := range []string{"pending", "processing", "delivery_scheduled", "setup_scheduled", "pickup_scheduled", "cancelled", "shipped", "paid_cancelled", "returned"} {
	for _, state := range []string{"pending", "delivery_scheduled", "setup_scheduled", "pickup_scheduled", "delivered", "followed_up"} {
		var state = state
		order.Scope(&admin.Scope{
			Name:  state,
			Label: strings.Title(strings.Replace(state, "_", " ", -1)),
			Group: "Order Status",
			Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
				return db.Where(orders.Order{Transition: transition.Transition{State: state}})
			},
		})
	}

	// filter by order source
	// order.Scope(&admin.Scope{
	// 	Name:  "Source",
	// 	Label: "订单来源",
	// 	Group: "Filter By Source",
	// 	Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
	// 		return db.Where(orders.Order{Source: "JD"})
	// 	},
	// })

	// filter by order source
	for _, item := range orders.SOURCES {
		var item = item
		order.Scope(&admin.Scope{
			Name:  item,
			Label: item,
			Group: "Filter By Source",
			Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
				return db.Where(orders.Order{Source: item})
			},
		})
	}

	// filter by order type
	// for _, item := range []string{"delivery", "setup", "delivery_and_setup", "repair", "clean", "sales"} {
	for _, item := range orders.ORDER_TYPES {
		var item = item // 这句必须有否则会报错，永远都是最后一个值
		order.Scope(&admin.Scope{
			Name:  item,
			Label: item,
			Group: "Filter By Order Type",
			Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
				// 两种写法都可以
				return db.Where(orders.Order{OrderType: item})
				// return db.Where("order_type = ?", item)
			},
		})
	}
}

func configureActions(Admin *admin.Admin, order *admin.Resource) {
	// 查看
	order.Action(&admin.Action{
		Name: "查看详情",
		URL: func(record interface{}, context *admin.Context) string {
			if order, ok := record.(*orders.Order); ok {
				return fmt.Sprintf("/admin/orders/%v", order.ID)
			}
			return "#"
		},
		URLOpenType: "_blank",
		Modes:       []string{"menu_item", "edit", "show"},
	})

	// define actions for Order
	type trackingNumberArgument struct {
		TrackingNumber string
	}

	type deliveryActionArgument struct {
		ManToDeliver string
	}

	type setupActionArgument struct {
		ManToSetup string
	}

	type createFollowUpActionArgument struct {
		// OrderID uint
		// OrderNo string
		// 对配送时效是否满意
		SatisfactionOfTimeliness string
		// 对服务态度是否满意
		SatisfactionOfServices string
		// 是否有开箱验货
		InspectTheGoods string
		// 师傅是否邀评
		RequestFeedback string
		// 是否有留下联系方式 方便后期有问题联系
		LeaveContactInfomation string
		// 师傅是否有介绍延保
		IntroduceWarrantyExtension string
		// 是否有把商品放到指定位置
		PositionProperly string
		// 有无问题要反馈
		Feedback string
		// 异常处理结果
		ExceptionHandling string
	}

	type createRatingActionArgument struct {
		// OrderID uint
		// OrderNo string
		// 好评还是差评
		Type string

		// 具体原因
		Reason string

		// 奖励或罚款金额
		Amount int

		Remark string
	}

	// type processingActionArgument struct {
	// 	ShippingFee float32
	// 	SetupFee    float32
	// 	PickupFee   float32
	// 	OrderType   string
	// }
	// processingActionResource := Admin.NewResource(&processingActionArgument{})
	// processingActionResource.Meta(&admin.Meta{
	// 	Name: "ShippingFee",
	// 	Type: "float",
	// })
	// processingActionResource.Meta(&admin.Meta{
	// 	Name: "SetupFee",
	// 	Type: "float",
	// })
	// processingActionResource.Meta(&admin.Meta{
	// 	Name: "PickupFee",
	// 	Type: "float",
	// })
	// processingActionResource.Meta(&admin.Meta{
	// 	Name:       "OrderType",
	// 	Type:       "select_one",
	// 	Collection: []string{"配送", "安装", "配送一体", "维修", "清洗"},
	// })
	// order.Action(&admin.Action{
	// 	Name: "Processing",
	// 	Handler: func(argument *admin.ActionArgument) error {
	// 		db := argument.Context.GetDB()
	// 		var (
	// 			tx  = argument.Context.GetDB().Begin()
	// 			arg = argument.Argument.(*processingActionArgument)
	// 		)
	// 		for _, record := range argument.FindSelectedRecords() {
	// 			order := record.(*orders.Order)
	// 			order.ShippingFee = arg.ShippingFee
	// 			order.SetupFee = arg.SetupFee
	// 			order.PickupFee = arg.PickupFee
	// 			order.OrderType = arg.OrderType
	// 			if err := orders.OrderState.Trigger("process", order, db); err != nil {
	// 				return err
	// 			}
	// 			if err := tx.Save(order).Error; err != nil {
	// 				tx.Rollback()
	// 				return err
	// 			}
	// 			tx.Commit()
	// 			return nil
	// 		}
	// 		return nil
	// 	},
	// 	Visible: func(record interface{}, context *admin.Context) bool {
	// 		if order, ok := record.(*orders.Order); ok {
	// 			return order.State == "pending"
	// 		}
	// 		return false
	// 	},
	// 	Resource: processingActionResource,
	// 	Modes:    []string{"show", "menu_item"},
	// })

	deliveryActionArgumentResource := Admin.NewResource(&deliveryActionArgument{})
	deliveryActionArgumentResource.Meta(&admin.Meta{
		Name: "ManToDeliver",
		Type: "select_one",
		Valuer: func(record interface{}, context *qor.Context) interface{} {
			// return record.(*users.User).ID
			return ""
		},
		Collection: func(value interface{}, context *qor.Context) (options [][]string) {
			var setupMen []users.User
			context.GetDB().Where("role = ?", "delivery_man").Find(&setupMen)
			for _, m := range setupMen {
				idStr := fmt.Sprintf("%d", m.ID)
				var option = []string{idStr, m.Name}
				options = append(options, option)
			}
			return options
		},
		// Collection: []string{"Male", "Female", "Unknown"},
	})
	// 安排配送
	order.Action(&admin.Action{
		Name: "Schedule Delivery",
		Handler: func(argument *admin.ActionArgument) error {
			var (
				tx = argument.Context.GetDB().Begin()
				// deliveryActionArgument = argument.Argument.(*deliveryActionArgument)
				arg = argument.Argument.(*deliveryActionArgument)
			)
			// if deliveryActionArgument.ManToDeliver != "" {
			for _, record := range argument.FindSelectedRecords() {
				order := record.(*orders.Order)
				order.ManToDeliverID = arg.ManToDeliver
				orders.OrderState.Trigger("schedule_delivery", order, tx, "man to deliver: "+arg.ManToDeliver)
				if err := tx.Save(order).Error; err != nil {
					tx.Rollback()
					return err
				}
			}
			// } else {
			// 	return errors.New("invalid man to deliver")
			// }
			tx.Commit()
			return nil
		},
		Visible: func(record interface{}, context *admin.Context) bool {
			if order, ok := record.(*orders.Order); ok {
				return order.State == "processing"
			}
			return false
		},
		// Resource: Admin.NewResource(&deliveryActionArgument{}),
		Resource: deliveryActionArgumentResource,
		Modes:    []string{"show", "edit"},
	})

	setupActionArgumentResource := Admin.NewResource(&setupActionArgument{})
	setupActionArgumentResource.Meta(&admin.Meta{
		Name: "ManToSetup",
		Type: "select_one",
		Valuer: func(record interface{}, context *qor.Context) interface{} {
			// return record.(*users.User).ID
			return ""
		},
		Collection: func(value interface{}, context *qor.Context) (options [][]string) {
			var setupMen []users.User
			context.GetDB().Where("role = ?", "setup_man").Find(&setupMen)
			for _, m := range setupMen {
				idStr := fmt.Sprintf("%d", m.ID)
				var option = []string{idStr, m.Name}
				options = append(options, option)
			}
			return options
		},
		// Collection: []string{"Male", "Female", "Unknown"},
	})

	// 安排安装
	order.Action(&admin.Action{
		Name: "Schedule Setup",
		Handler: func(argument *admin.ActionArgument) error {
			var (
				tx  = argument.Context.GetDB().Begin()
				arg = argument.Argument.(*setupActionArgument)
			)
			// if setupArgument.ManToSetup != "" {
			for _, record := range argument.FindSelectedRecords() {
				order := record.(*orders.Order)
				order.ManToSetupID = arg.ManToSetup
				orders.OrderState.Trigger("schedule_setup", order, tx, "man to setup: "+arg.ManToSetup)
				if err := tx.Save(order).Error; err != nil {
					tx.Rollback()
					return err
				}
			}
			// } else {
			// 	return errors.New("invalid man to setup")
			// }
			tx.Commit()
			return nil
		},
		Visible: func(record interface{}, context *admin.Context) bool {
			if order, ok := record.(*orders.Order); ok {
				return order.State == "processing"
			}
			return false
		},
		// Resource: Admin.NewResource(&setupActionArgument{}),
		Resource: setupActionArgumentResource,
		Modes:    []string{"show", "menu_item"},
	})

	// order.Action(&admin.Action{
	// 	Name: "创建回访记录",
	// 	URL: func(record interface{}, context *admin.Context) string {
	// 		if order, ok := record.(*orders.Order); ok {
	// 			return fmt.Sprintf("/admin/order_follow_ups/new?order_id=%v&order_no=%v", order.ID, order.OrderNo)
	// 		}
	// 		return "#"
	// 	},
	// 	Modes: []string{"menu_item", "edit", "show"},
	// })

	followUpResource := Admin.NewResource(&createFollowUpActionArgument{})
	followUpResource.Meta(&admin.Meta{
		Name:       "SatisfactionOfTimeliness",
		Type:       "select_one",
		Collection: []string{"是", "否"},
	})

	followUpResource.Meta(&admin.Meta{
		Name:       "SatisfactionOfServices",
		Type:       "select_one",
		Collection: []string{"是", "否"},
	})

	followUpResource.Meta(&admin.Meta{
		Name:       "InspectTheGoods",
		Type:       "select_one",
		Collection: []string{"是", "否"},
	})

	followUpResource.Meta(&admin.Meta{
		Name:       "RequestFeedback",
		Type:       "select_one",
		Collection: []string{"是", "否"},
	})

	followUpResource.Meta(&admin.Meta{
		Name:       "LeaveContactInfomation",
		Type:       "select_one",
		Collection: []string{"是", "否"},
	})

	followUpResource.Meta(&admin.Meta{
		Name:       "IntroduceWarrantyExtension",
		Type:       "select_one",
		Collection: []string{"是", "否"},
	})

	followUpResource.Meta(&admin.Meta{
		Name:       "PositionProperly",
		Type:       "select_one",
		Collection: []string{"是", "否"},
	})
	order.Action(&admin.Action{
		Name: "创建回访",
		Handler: func(argument *admin.ActionArgument) error {
			var (
				tx  = argument.Context.GetDB().Begin()
				arg = argument.Argument.(*createFollowUpActionArgument)
			)

			// var order orders.Order
			// tx.Model(orders.Order{}).Where("id = ?", argument.PrimaryValues[0]).Find(&order)
			// fmt.Println(argument.Context)
			// fmt.Println(argument.PrimaryValues)
			var followUp orders.OrderFollowUp
			// if order.OrderNo != "" {
			for _, record := range argument.FindSelectedRecords() {
				item := record.(*orders.Order)
				// fmt.Println(item)
				// fmt.Println(arg)
				followUp.OrderID = item.ID
				followUp.OrderNo = item.OrderNo
				followUp.SatisfactionOfTimeliness = arg.SatisfactionOfTimeliness
				followUp.SatisfactionOfServices = arg.SatisfactionOfServices
				followUp.InspectTheGoods = arg.InspectTheGoods
				followUp.RequestFeedback = arg.RequestFeedback
				followUp.LeaveContactInfomation = arg.LeaveContactInfomation
				followUp.IntroduceWarrantyExtension = arg.IntroduceWarrantyExtension
				followUp.PositionProperly = arg.PositionProperly
				followUp.Feedback = arg.Feedback
				followUp.ExceptionHandling = arg.ExceptionHandling
				if err := tx.Where(orders.OrderFollowUp{OrderNo: item.OrderNo}).FirstOrCreate(&followUp).Error; err != nil {
					tx.Rollback()
					return err
				}
			}
			// } else {
			// 	return errors.New("create follow up failed")
			// }
			tx.Commit()
			return nil
		},
		Visible: func(record interface{}, context *admin.Context) bool {
			// if order, ok := record.(*orders.Order); ok {
			// 	return order.State == "processing"
			// }
			// return false
			return true
		},
		// Resource: Admin.NewResource(&setupActionArgument{}),
		Resource: followUpResource,
		Modes:    []string{"show", "menu_item"},
	})

	ratingResource := Admin.NewResource(&createRatingActionArgument{})
	ratingResource.Meta(&admin.Meta{
		Name:       "Type",
		Type:       "select_one",
		Collection: []string{"好评", "差评"},
	})
	order.Action(&admin.Action{
		Name: "创建评价",
		Handler: func(argument *admin.ActionArgument) error {
			var (
				tx  = argument.Context.GetDB().Begin()
				arg = argument.Argument.(*createRatingActionArgument)
			)
			var rating orders.Rating
			// if order.OrderNo != "" {
			for _, record := range argument.FindSelectedRecords() {
				item := record.(*orders.Order)
				rating.OrderID = item.ID
				rating.Type = arg.Type
				rating.Reason = arg.Reason
				rating.Amount = arg.Amount
				rating.Remark = arg.Remark

				if err := tx.Save(&rating).Error; err != nil {
					tx.Rollback()
					return err
				}
			}
			// } else {
			// 	return errors.New("create follow up failed")
			// }
			tx.Commit()
			return nil
		},
		Visible: func(record interface{}, context *admin.Context) bool {
			return true
		},
		Resource: ratingResource,
		Modes:    []string{"show", "menu_item"},
	})
}

// 废弃订单
func configureAbandonedOrders(Admin *admin.Admin) {
	// Define another resource for same model
	abandonedOrder := Admin.AddResource(&orders.Order{}, &admin.Config{Name: "Abandoned Order", Menu: []string{"Order Management"}})
	abandonedOrder.Meta(&admin.Meta{Name: "ShippingAddress", Type: "single_edit"})
	abandonedOrder.Meta(&admin.Meta{Name: "BillingAddress", Type: "single_edit"})

	// Define default scope for abandoned orders
	abandonedOrder.Scope(&admin.Scope{
		Default: true,
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("abandoned_reason IS NOT NULL AND abandoned_reason <> ?", "")
		},
	})

	// Define scopes for abandoned orders
	for _, amount := range []int{5000, 10000, 20000} {
		var amount = amount
		abandonedOrder.Scope(&admin.Scope{
			Name:  fmt.Sprint(amount),
			Group: "Amount Greater Than",
			Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
				return db.Where("payment_amount > ?", amount)
			},
		})
	}

	abandonedOrder.IndexAttrs("-ShippingAddress", "-BillingAddress", "-DiscountValue", "-OrderItems")
	abandonedOrder.NewAttrs("-DiscountValue")
	abandonedOrder.EditAttrs("-DiscountValue")
	abandonedOrder.ShowAttrs("-DiscountValue")
}

func configureScopesForOrderItems(model *admin.Resource) {
	var cats []string
	db.DB.Model(&orders.OrderItem{}).Pluck("DISTINCT category", &cats)
	// db.DB.Find(&Client{}).Pluck("DISTINCT phone_number", &phoneNumbers)

	for _, cate := range cats {
		var state = cate
		model.Scope(&admin.Scope{
			Name:  state,
			Label: state,
			Group: "Filter By Category",
			Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
				return db.Where("category = ?", state)
			},
		})
	}
	model.Scope(&admin.Scope{
		Name:  "空",
		Label: "无",
		Group: "Filter By Category",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("category = ?", "")
		},
	})

	model.Scope(&admin.Scope{
		Name:  "NA",
		Label: "未标运费",
		Group: "未标运费",
		Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
			return db.Where("delivery_fee = ?", 0)
		},
	})

}

func configureScopesForRules(model *admin.Resource) {
	var cats []string
	db.DB.Model(&orders.Rule{}).Pluck("DISTINCT category", &cats)
	// db.DB.Find(&Client{}).Pluck("DISTINCT phone_number", &phoneNumbers)

	for _, cate := range cats {
		var state = cate
		model.Scope(&admin.Scope{
			Name:  state,
			Label: state,
			Group: "Filter By Category",
			Handler: func(db *gorm.DB, context *qor.Context) *gorm.DB {
				return db.Where("category = ?", state)
			},
		})
	}
}
