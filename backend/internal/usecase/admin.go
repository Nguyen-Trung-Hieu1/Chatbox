package usecase

import (
	"backend/internal/domain"
	"context"
	"sort"
	"time"
)

type AdminRepository interface {
	ListAccounts(context.Context) ([]domain.AdminAccount, error)
	Usage(context.Context, string) ([]domain.UsageRecord, error)
	SetPlan(context.Context, string, string, time.Time, int64) (*domain.AdminAccount, error)
}

type Admin struct {
	repo        AdminRepository
	freeCredits int64
}

func NewAdmin(repo AdminRepository, freeCredits int64) *Admin {
	return &Admin{repo: repo, freeCredits: freeCredits}
}

func (a *Admin) SetPlan(ctx context.Context, userID, plan string) (*domain.AdminAccount, error) {
	if plan != "free" && plan != "paid" {
		return nil, domain.ErrInvalidPlan
	}
	credits := a.freeCredits
	if plan == "paid" {
		credits = 2000
	}
	return a.repo.SetPlan(ctx, userID, plan, time.Now().UTC(), credits)
}

func (a *Admin) Users(ctx context.Context) ([]domain.AdminAccount, error) {
	items, err := a.repo.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := range items {
		if items[i].Plan != "paid" || items[i].PaidUntil == nil || !items[i].PaidUntil.After(now) {
			items[i].Plan, items[i].PaidUntil = "free", nil
		}
	}
	return items, nil
}

func (a *Admin) Usage(ctx context.Context, id string) (*domain.AdminUsageReport, error) {
	records, err := a.repo.Usage(ctx, id)
	if err != nil {
		return nil, err
	}
	return summarizeUsage(records), nil
}

func summarizeUsage(records []domain.UsageRecord) *domain.AdminUsageReport {
	report := &domain.AdminUsageReport{Days: []domain.DailyUsage{}, RecordLimit: 1000}
	days := map[string]*domain.DailyUsage{}
	models := map[string]int64{}
	ict := time.FixedZone("ICT", 7*60*60)
	for _, record := range records {
		date := record.CreatedAt.In(ict).Format("2006-01-02")
		day := days[date]
		if day == nil {
			day = &domain.DailyUsage{Date: date}
			days[date] = day
		}
		in, out := int64(record.InputTokens), int64(record.OutputTokens)
		if record.Kind == "api" {
			day.APIInput += in
			day.APIOutput += out
		} else {
			day.WebInput += in
			day.WebOutput += out
		}
		models[record.Model] += in + out
	}
	for _, day := range days {
		report.Days = append(report.Days, *day)
	}
	sort.Slice(report.Days, func(i, j int) bool { return report.Days[i].Date > report.Days[j].Date })
	for id, tokens := range models {
		if report.TopModel == nil || tokens > report.TopModel.Tokens || (tokens == report.TopModel.Tokens && id < report.TopModel.ID) {
			report.TopModel = &domain.ModelUsage{ID: id, Tokens: tokens}
		}
	}
	return report
}
