package reference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Currency struct {
	Code          string
	Name          string
	Symbol        string
	DecimalPlaces int16
}

type Country struct {
	Code          string
	Name          string
	CurrencyCode  string
	DefaultLocale string
}

type Locale struct {
	Code     string
	Language string
	Country  string
	Extra    bool
}

type Result struct {
	Seed        int64
	Currencies  int64
	Countries   int64
	Locales     int64
	Fingerprint string
}

var currencies = []Currency{
	{Code: "BRL", Name: "Brazilian Real", Symbol: "R$", DecimalPlaces: 2},
	{Code: "USD", Name: "US Dollar", Symbol: "$", DecimalPlaces: 2},
	{Code: "EUR", Name: "Euro", Symbol: "€", DecimalPlaces: 2},
	{Code: "JPY", Name: "Japanese Yen", Symbol: "¥", DecimalPlaces: 0},
	{Code: "GBP", Name: "Pound Sterling", Symbol: "£", DecimalPlaces: 2},
	{Code: "ARS", Name: "Argentine Peso", Symbol: "$", DecimalPlaces: 2},
	{Code: "CAD", Name: "Canadian Dollar", Symbol: "$", DecimalPlaces: 2},
	{Code: "AUD", Name: "Australian Dollar", Symbol: "$", DecimalPlaces: 2},
	{Code: "CLP", Name: "Chilean Peso", Symbol: "$", DecimalPlaces: 0},
	{Code: "MXN", Name: "Mexican Peso", Symbol: "$", DecimalPlaces: 2},
	{Code: "CHF", Name: "Swiss Franc", Symbol: "CHF", DecimalPlaces: 2},
	{Code: "INR", Name: "Indian Rupee", Symbol: "₹", DecimalPlaces: 2},
}

var countries = []Country{
	{Code: "BR", Name: "Brazil", CurrencyCode: "BRL", DefaultLocale: "pt-BR"},
	{Code: "US", Name: "United States", CurrencyCode: "USD", DefaultLocale: "en-US"},
	{Code: "DE", Name: "Germany", CurrencyCode: "EUR", DefaultLocale: "de-DE"},
	{Code: "JP", Name: "Japan", CurrencyCode: "JPY", DefaultLocale: "ja-JP"},
	{Code: "GB", Name: "United Kingdom", CurrencyCode: "GBP", DefaultLocale: "en-GB"},
	{Code: "AR", Name: "Argentina", CurrencyCode: "ARS", DefaultLocale: "es-AR"},
	{Code: "CA", Name: "Canada", CurrencyCode: "CAD", DefaultLocale: "en-CA"},
	{Code: "AU", Name: "Australia", CurrencyCode: "AUD", DefaultLocale: "en-AU"},
	{Code: "CL", Name: "Chile", CurrencyCode: "CLP", DefaultLocale: "es-CL"},
	{Code: "MX", Name: "Mexico", CurrencyCode: "MXN", DefaultLocale: "es-MX"},
	{Code: "CH", Name: "Switzerland", CurrencyCode: "CHF", DefaultLocale: "de-CH"},
	{Code: "IN", Name: "India", CurrencyCode: "INR", DefaultLocale: "en-IN"},
	{Code: "PT", Name: "Portugal", CurrencyCode: "EUR", DefaultLocale: "pt-PT"},
}

var extraLocales = []Locale{
	{Code: "es-US", Language: "es", Country: "US", Extra: true},
	{Code: "fr-CA", Language: "fr", Country: "CA", Extra: true},
	{Code: "fr-CH", Language: "fr", Country: "CH", Extra: true},
	{Code: "it-CH", Language: "it", Country: "CH", Extra: true},
	{Code: "hi-IN", Language: "hi", Country: "IN", Extra: true},
}

func Seed(ctx context.Context, pool *pgxpool.Pool, seed int64) (Result, error) {
	if pool == nil {
		return Result{}, fmt.Errorf("seed: nil pool")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("seed: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, currency := range currencies {
		if _, err := tx.Exec(ctx, `
			INSERT INTO currencies (code, name, symbol, decimal_places)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code) DO UPDATE
			SET name = EXCLUDED.name,
			    symbol = EXCLUDED.symbol,
			    decimal_places = EXCLUDED.decimal_places,
			    updated_at = now()
		`, currency.Code, currency.Name, currency.Symbol, currency.DecimalPlaces); err != nil {
			return Result{}, fmt.Errorf("seed: upsert currency %s: %w", currency.Code, err)
		}
	}

	for _, country := range countries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO countries (code, name, currency_code, default_locale)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code) DO UPDATE
			SET name = EXCLUDED.name,
			    currency_code = EXCLUDED.currency_code,
			    default_locale = EXCLUDED.default_locale,
			    updated_at = now()
		`, country.Code, country.Name, country.CurrencyCode, country.DefaultLocale); err != nil {
			return Result{}, fmt.Errorf("seed: upsert country %s: %w", country.Code, err)
		}
	}

	selected := selectedLocales(seed)
	for _, locale := range canonicalLocales() {
		if err := upsertLocale(ctx, tx, locale); err != nil {
			return Result{}, err
		}
	}
	codes := make([]string, 0, len(selected))
	for _, locale := range selected {
		if err := upsertLocale(ctx, tx, locale); err != nil {
			return Result{}, err
		}
		codes = append(codes, locale.Code)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM locales
		WHERE (is_extra OR code = ANY($1::text[]))
		  AND NOT (code = ANY($2::text[]))
	`, extraLocaleCodes(), codes); err != nil {
		return Result{}, fmt.Errorf("seed: remove stale extra locales: %w", err)
	}

	result := Result{Seed: seed, Fingerprint: Fingerprint(seed)}
	if result.Currencies, err = count(ctx, tx, "currencies"); err != nil {
		return Result{}, err
	}
	if result.Countries, err = count(ctx, tx, "countries"); err != nil {
		return Result{}, err
	}
	if result.Locales, err = count(ctx, tx, "locales"); err != nil {
		return Result{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("seed: commit: %w", err)
	}
	return result, nil
}

func upsertLocale(ctx context.Context, tx pgx.Tx, locale Locale) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO locales (code, language, country_code, is_extra)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (code) DO UPDATE
		SET language = EXCLUDED.language,
		    country_code = EXCLUDED.country_code,
		    is_extra = EXCLUDED.is_extra,
		    updated_at = now()
	`, locale.Code, locale.Language, locale.Country, locale.Extra)
	if err != nil {
		return fmt.Errorf("seed: upsert locale %s: %w", locale.Code, err)
	}
	return nil
}

func count(ctx context.Context, tx pgx.Tx, table string) (int64, error) {
	var total int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&total); err != nil {
		return 0, fmt.Errorf("seed: count %s: %w", table, err)
	}
	return total, nil
}

func extraLocaleCodes() []string {
	codes := make([]string, 0, len(extraLocales))
	for _, locale := range extraLocales {
		codes = append(codes, locale.Code)
	}
	return codes
}

func canonicalLocales() []Locale {
	locales := make([]Locale, 0, len(countries))
	for _, country := range countries {
		locales = append(locales, Locale{
			Code:     country.DefaultLocale,
			Language: language(country.DefaultLocale),
			Country:  country.Code,
		})
	}
	return locales
}

func selectedLocales(seed int64) []Locale {
	if len(extraLocales) == 0 {
		return nil
	}
	total := 2 + int(mod(seed, 3))
	start := int(mod(seed, int64(len(extraLocales))))
	selected := make([]Locale, 0, total)
	for i := 0; i < total; i++ {
		selected = append(selected, extraLocales[(start+i)%len(extraLocales)])
	}
	return selected
}

func Fingerprint(seed int64) string {
	hash := sha256.New()
	for _, currency := range currencies {
		_, _ = fmt.Fprintf(hash, "currency|%s|%s|%s|%d\n", currency.Code, currency.Name, currency.Symbol, currency.DecimalPlaces)
	}
	for _, country := range countries {
		_, _ = fmt.Fprintf(hash, "country|%s|%s|%s|%s\n", country.Code, country.Name, country.CurrencyCode, country.DefaultLocale)
	}
	locales := append(canonicalLocales(), selectedLocales(seed)...)
	sort.Slice(locales, func(i, j int) bool { return locales[i].Code < locales[j].Code })
	for _, locale := range locales {
		_, _ = fmt.Fprintf(hash, "locale|%s|%s|%s|%t\n", locale.Code, locale.Language, locale.Country, locale.Extra)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func language(code string) string {
	return strings.ToLower(strings.SplitN(code, "-", 2)[0])
}

func mod(value, divisor int64) int64 {
	result := value % divisor
	if result < 0 {
		result += divisor
	}
	return result
}
