package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"price-feeder/oracle/types"

	"github.com/rs/zerolog"
)

var (
	_                         Provider = (*AstroportProvider)(nil)
	astroportDefaultEndpoints          = Endpoint{
		Name:         ProviderAstroport,
		Rest:         "https://develop-multichain-api.astroport.fi",
		PollInterval: 10 * time.Second,
	}
)

type (
	// AstroportProvider defines an oracle provider implemented by the
	// Astroport API.

	AstroportProvider struct {
		provider
	}

	AstroportQuery struct {
		Query string `json:"query"`
	}

	AstroportTokensResponse struct {
		Data AstroportTokensData `json:"data"`
	}

	AstroportTokensData struct {
		Tokens []AstroportToken `json:"tokens"`
	}

	AstroportToken struct {
		Address  string  `json:"tokenAddr"`
		Symbol   string  `json:"symbol"`
		Price    float64 `json:"priceUsd"`
		Decimals float64 `json:"decimals"`
	}

	AstroportPoolsResponse struct {
		Data AstroportPoolsData `json:"data"`
	}

	AstroportPoolsData struct {
		Pools []AstroportPool `json:"pools"`
	}

	AstroportPool struct {
		Volume    float64          `json:"dayVolumeUsd"`
		Assets    []AstroportAsset `json:"assets"`
		Liquidity float64          `json:"poolLiquidityUsd"`
	}

	AstroportAsset struct {
		Address string `json:"address"`
		Amount  string `json:"amount"`
		Symbol  string `json:"symbol"`
	}
)

func NewAstroportProvider(
	ctx context.Context,
	logger zerolog.Logger,
	endpoints Endpoint,
	pairs ...types.CurrencyPair,
) (*AstroportProvider, error) {
	provider := &AstroportProvider{}
	provider.Init(
		ctx,
		endpoints,
		logger,
		pairs,
		nil,
		nil,
	)
	go startPolling(provider, provider.endpoints.PollInterval, logger)
	return provider, nil
}

func (p *AstroportProvider) Poll() error {
	url := p.endpoints.Rest + "/graphql"

	tokenQuery := AstroportQuery{
		Query: `query { tokens(chains: "phoenix-1") { priceUsd symbol tokenAddr } }`,
	}

	poolQuery := AstroportQuery{
		Query: `query { pools(chains: "phoenix-1") { poolLiquidityUsd dayVolumeUsd assets { symbol address amount } } }`,
	}

	data, err := json.Marshal(tokenQuery)
	if err != nil {
		return err
	}

	content, err := p.httpPost(url, data)
	if err != nil {
		fmt.Println(string(content))
		return err
	}

	var tokensResponse AstroportTokensResponse
	err = json.Unmarshal(content, &tokensResponse)
	if err != nil {
		return err
	}

	data, err = json.Marshal(poolQuery)
	if err != nil {
		return err
	}

	content, err = p.httpPost(url, data)
	if err != nil {
		fmt.Println(string(content))
		return err
	}

	var poolsResponse AstroportPoolsResponse
	err = json.Unmarshal(content, &poolsResponse)
	if err != nil {
		return err
	}

	// Create a map of 'real' assets, as there are other pools with the same
	// symbol that are not the Terra native LUNA for example

	tokens := map[string]AstroportToken{}

	whitelist := map[string]string{
		"USDC": "ibc/b3504e092456ba618cc28ac671a71fb08c6ca0fd0be7c8a5b5a3e2dd933cc9e4",
		"LUNA": "uluna",
	}

	for _, token := range tokensResponse.Data.Tokens {
		address, ok := whitelist[token.Symbol]
		if ok && token.Address != address {
			continue
		}

		tokens[token.Address] = token
	}

	p.mtx.Lock()
	defer p.mtx.Unlock()

	timestamp := time.Now()

	for _, pool := range poolsResponse.Data.Pools {
		// ignore pools with a liquidity < $ 100k
		if pool.Liquidity < 100000 {
			continue
		}

		poolAsset1, poolAsset2, ok := p.getPoolAssets(pool)
		if !ok {
			continue
		}

		// ignore pools with non whitelisted assets
		token1, ok := tokens[strings.ToLower(poolAsset1.Address)]
		if !ok {
			continue
		}

		token2, ok := tokens[strings.ToLower(poolAsset2.Address)]
		if !ok {
			continue
		}

		symbol := strings.ToUpper(poolAsset1.Symbol + poolAsset2.Symbol)

		price1 := floatToDec(token1.Price)
		price2 := floatToDec(token2.Price)

		p.tickers[symbol] = types.TickerPrice{
			Price:  price1.Quo(price2),
			Volume: floatToDec(pool.Volume).Quo(price1),
			Time:   timestamp,
		}
	}

	return nil
}

func (p *AstroportProvider) getPoolAssets(pool AstroportPool) (AstroportAsset, AstroportAsset, bool) {
	// check if A/B or B/A matches a defined base/quote pair and return
	// assets and true in correct order, empty assets and false otherwise

	a1 := pool.Assets[0]
	a2 := pool.Assets[1]
	_, ok := p.pairs[strings.ToUpper(a1.Symbol+a2.Symbol)]
	if ok {
		return a1, a2, true
	}

	_, ok = p.pairs[strings.ToUpper(a2.Symbol+a1.Symbol)]
	if ok {
		return a2, a1, true
	}

	return AstroportAsset{}, AstroportAsset{}, false
}
