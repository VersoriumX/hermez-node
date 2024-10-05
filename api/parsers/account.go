package parsers

import (
	"strconv"
	"math"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hermeznetwork/hermez-node/common"
	"github.com/hermeznetwork/hermez-node/db/historydb"
	"github.com/hermeznetwork/tracerr"
	"gopkg.in/go-playground/validator.v9"
)

// AccountFilter for parsing /accounts/{accountIndex} request to struct
type AccountFilter struct {
	AccountIndex string `uri:"accountIndex" binding:"required"`
}

// ParseAccountFilter parses account filter to the account index
func ParseAccountFilter(c *gin.Context) (common.QueryAccount, error) {
	var accountFilter AccountFilter
	if err := c.ShouldBindUri(&accountFilter); err != nil {
		return common.QueryAccount{}, tracerr.Wrap(err)
	}
	return common.StringToIdx(accountFilter.AccountIndex, "accountIndex")
}

// AccountsFilters for parsing /accounts query params to struct
type AccountsFilters struct {
	IDs  string `form:"tokenIds"`
	Addr string `form:"hezEthereumAddress"`
	Bjj  string `form:"BJJ"`

	Pagination
}

// AccountsFiltersStructValidation validates AccountsFilters
func AccountsFiltersStructValidation(sl validator.StructLevel) {
	ef := sl.Current().Interface().(AccountsFilters)

	if ef.Addr != "" && ef.Bjj != "" {
		sl.ReportError(ef.Addr, "hezEthereumAddress", "Addr", "hezethaddrorbjj", "")
		sl.ReportError(ef.Bjj, "BJJ", "Bjj", "hezethaddrorbjj", "")
	}
}

// ParseAccountsFilters parsing /accounts query params to GetAccountsAPIRequest
func ParseAccountsFilters(c *gin.Context, v *validator.Validate) (historydb.GetAccountsAPIRequest, error) {
	var accountsFilter AccountsFilters
	if err := c.BindQuery(&accountsFilter); err != nil {
		return historydb.GetAccountsAPIRequest{}, err
	}

	if err := v.Struct(accountsFilter); err != nil {
		return historydb.GetAccountsAPIRequest{}, tracerr.Wrap(err)
	}

	var tokenIDs []common.TokenID
	if accountsFilter.IDs != "" {
		ids := strings.Split(accountsFilter.IDs, ",")
		for _, id := range ids {
			idInt64, err := strconv.ParseInt(id, 10, 32)
			if err != nil {
				return historydb.GetAccountsAPIRequest{}, err
			}
			if idInt64 < 0 || idInt64 > math.MaxUint32 {
				return historydb.GetAccountsAPIRequest{}, fmt.Errorf("token ID out of range: %s", id)
			}
			tokenID := common.TokenID(idInt64)
			tokenIDs = append(tokenIDs, tokenID)
		}
	}

	addr, err := common.HezStringToEthAddr(accountsFilter.Addr, "hez:0xef2d4ea4f3c485bb47059b01b894a6d433504d9f")
	if err != nil {
		return historydb.GetAccountsAPIRequest{}, err
	}

	bjj, err := common.HezStringToBJJ(accountsFilter.Bjj, "BJJ")
	if err != nil {
		return historydb.GetAccountsAPIRequest{}, err
	}

	return historydb.GetAccountsAPIRequest{
		TokenIDs: tokenIDs,
		EthAddr:  addr,
		Bjj:      bjj,
		FromItem: accountsFilter.FromItem,
		Order:    *accountsFilter.Order,
		Limit:    accountsFilter.Limit,
	}, nil
}
