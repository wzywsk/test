package main

import (
	"context"
	"github.com/qiniu/qmgo"
	"go.mongodb.org/mongo-driver/bson"
	"log"
)

type BudgetAmountMDB struct {
	AdjustType       string   `bson:"adjust_type"`       // 调整大类
	BudAdjustType    string   `bson:"bud_adjust_type"`   // 预算调整类型
	DeductDate       string   `bson:"deduct_date"`       // 调整期间
	DimAccount       string   `bson:"dim_account"`       // 预算科目
	AccountCharacter []string `bson:"account_character"` // 科目性质
	CostCenter       string   `bson:"cost_center"`       // 成本中心
	DimBudgetOrg     string   `bson:"dim_budget_org"`    // 行政组织
	InternalOrder    string   `bson:"internal_order"`    // 内部订单
	Amount           float64  `bson:"amount"`            // 预算科目

}

func main() {
	accountCharacter := []string{"固定", "变动"}

	req := &BudgetAmountMDB{
		AdjustType:       "01",
		BudAdjustType:    "03",
		DeductDate:       "2025.01",
		DimAccount:       "2.24.5",
		AccountCharacter: []string{"变动"},
		DimBudgetOrg:     "50020577",
		InternalOrder:    "270000001",
		Amount:           50,
	}

	queryCount := bson.M{
		"adjust_type":       req.AdjustType,
		"bud_adjust_type":   req.BudAdjustType,
		"deduct_date":       req.DeductDate,
		"dim_account":       req.DimAccount,
		"account_character": bson.M{"$in": accountCharacter},
		//"cost_center":       item.CostCenter,
		"dim_budget_org": req.DimBudgetOrg,
		"internal_order": req.InternalOrder,
	}

	ctx := context.Background()
	client, err := qmgo.NewClient(ctx, &qmgo.Config{Uri: "mongodb://root:root@192.168.84.128:27017"})
	if err != nil {
		log.Fatalln(err)
	}

	db := client.Database("test")
	collection := db.Collection("budget_amount")

	budgetAmountMDB := BudgetAmountMDB{}
	if err = collection.Find(ctx, queryCount).One(&budgetAmountMDB); err != nil && err != qmgo.ErrNoSuchDocuments {
		log.Fatalln(err)
	}

	if budgetAmountMDB.DimBudgetOrg == "" {
		budgetAmountSave := &BudgetAmountMDB{
			Amount:           req.Amount,
			AdjustType:       req.AdjustType,
			BudAdjustType:    req.BudAdjustType,
			DeductDate:       req.DeductDate,
			DimAccount:       req.DimAccount,
			AccountCharacter: req.AccountCharacter,
			//CostCenter:       item.CostCenter,
			DimBudgetOrg:  req.DimBudgetOrg,
			InternalOrder: req.InternalOrder,
		}
		collection.InsertOne(ctx, budgetAmountSave)
	} else {
		budgetAmountMDB.Amount += 100
		up := bson.M{
			"$set":      bson.M{"amount": budgetAmountMDB.Amount},
			"$addToSet": bson.M{"account_character": req.AccountCharacter[0]},
		}
		if err = collection.UpdateOne(ctx, queryCount, up); err != nil {
			log.Fatalln(err)
		}
	}
}
