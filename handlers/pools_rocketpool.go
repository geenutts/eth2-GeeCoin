package handlers

import (
	"bytes"
	"encoding/json"
	"eth2-exporter/db"
	"eth2-exporter/types"
	"eth2-exporter/utils"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

var poolsRocketpoolTemplate = template.Must(template.New("rocketpool").Funcs(utils.GetTemplateFuncs()).ParseFiles("templates/layout.html", "templates/pools_rocketpool.html"))

// PoolsRocketpool returns the rocketpool using a go template
func PoolsRocketpool(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	data := InitPageData(w, r, "pools/rocketpool", "/pools/rocketpool", "Rocketpool")
	data.HeaderAd = true

	err := poolsRocketpoolTemplate.ExecuteTemplate(w, "layout", data)

	if err != nil {
		logger.Errorf("error executing template for %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", 503)
		return
	}
}

func PoolsRocketpoolDataMinipools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()
	draw, err := strconv.ParseUint(q.Get("draw"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables data parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	start, err := strconv.ParseUint(q.Get("start"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables start parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	length, err := strconv.ParseUint(q.Get("length"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables length parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	if length > 100 {
		length = 100
	}
	search := strings.Replace(q.Get("search[value]"), "0x", "", -1)
	if len(search) > 128 {
		search = search[:128]
	}

	orderColumn := q.Get("order[0][column]")
	orderByMap := map[string]string{
		"0": "address",
		"1": "pubkey",
		"2": "node_address",
		"3": "node_fee",
		"4": "deposit_type",
		"5": "status",
	}
	orderBy, exists := orderByMap[orderColumn]
	if !exists {
		orderBy = "address"
	}
	orderDir := q.Get("order[0][dir]")
	if orderDir != "desc" && orderDir != "asc" {
		orderDir = "desc"
	}

	recordsTotal := uint64(0)
	recordsFiltered := uint64(0)
	var minipools []types.RocketpoolPageDataMinipool
	if search == "" {
		err = db.DB.Select(&minipools, fmt.Sprintf(`
			select 
				rocketpool_minipools.*, 
				validators.validatorindex as validator_index,
				coalesce(validator_names.name,'') as validator_name,
				cnt.total_count
			from rocketpool_minipools
			left join validator_names on rocketpool_minipools.pubkey = validator_names.publickey
			left join validators on rocketpool_minipools.pubkey = validators.pubkey
			left join (select count(*) from rocketpool_minipools) cnt(total_count) ON true
			order by %s %s
			limit $1
			offset $2`, orderBy, orderDir), length, start)
		if err != nil {
			logger.Errorf("error getting rocketpool-minipools from db: %v", err)
			http.Error(w, "Internal server error", 503)
			return
		}
	} else {
		err = db.DB.Select(&minipools, fmt.Sprintf(`
			with matched_minipools as (
				select address from rocketpool_minipools where encode(pubkey::bytea,'hex') like $3
				union select address from rocketpool_minipools where encode(address::bytea,'hex') like $3
				union (select address from validator_names inner join rocketpool_minipools on rocketpool_minipools.pubkey = validator_names.publickey where name ilike $4)
			)
			select 
				rocketpool_minipools.*, 
				validators.validatorindex as validator_index,
				coalesce(validator_names.name,'') as validator_name,
				cnt.total_count
			from rocketpool_minipools
			inner join matched_minipools on rocketpool_minipools.address = matched_minipools.address
			left join validator_names on rocketpool_minipools.pubkey = validator_names.publickey
			left join validators on rocketpool_minipools.pubkey = validators.pubkey
			left join (select count(*) from matched_minipools) cnt(total_count) ON true
			order by %s %s
			limit $1
			offset $2`, orderBy, orderDir), length, start, search+"%", "%"+search+"%")
		if err != nil {
			logger.Errorf("error getting rocketpool-minipools from db (with search: %v): %v", search, err)
			http.Error(w, "Internal server error", 503)
			return
		}
	}

	if len(minipools) > 0 {
		recordsTotal = minipools[0].TotalCount
		recordsFiltered = minipools[0].TotalCount
	}

	tableData := make([][]interface{}, 0, len(minipools))
	zeroAddr := make([]byte, 48)

	for _, row := range minipools {
		entry := []interface{}{}
		entry = append(entry, utils.FormatEth1Address(row.Address))
		if c := bytes.Compare(row.Pubkey, zeroAddr); c == 0 {
			entry = append(entry, "N/A")
		} else {
			entry = append(entry, utils.FormatValidatorWithName(row.Pubkey, row.ValidatorName))
		}
		entry = append(entry, utils.FormatEth1Address(row.NodeAddress))
		entry = append(entry, row.NodeFee)
		entry = append(entry, row.DepositType)
		entry = append(entry, row.Status)
		tableData = append(tableData, entry)
	}

	data := &types.DataTableResponse{
		Draw:            draw,
		RecordsTotal:    recordsTotal,
		RecordsFiltered: recordsFiltered,
		Data:            tableData,
	}

	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Errorf("error enconding json response for %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", 503)
		return
	}
}

func PoolsRocketpoolDataNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()
	draw, err := strconv.ParseUint(q.Get("draw"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables data parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	start, err := strconv.ParseUint(q.Get("start"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables start parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	length, err := strconv.ParseUint(q.Get("length"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables length parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	if length > 100 {
		length = 100
	}
	search := strings.Replace(q.Get("search[value]"), "0x", "", -1)
	if len(search) > 128 {
		search = search[:128]
	}

	orderColumn := q.Get("order[0][column]")
	orderByMap := map[string]string{
		"0": "address",
		"1": "timezone_location",
		"2": "rpl_stake",
		"3": "min_rpl_stake",
		"4": "max_rpl_stake",
	}
	orderBy, exists := orderByMap[orderColumn]
	if !exists {
		orderBy = "address"
	}
	orderDir := q.Get("order[0][dir]")
	if orderDir != "desc" && orderDir != "asc" {
		orderDir = "desc"
	}

	recordsTotal := uint64(0)
	recordsFiltered := uint64(0)
	var dbResult []types.RocketpoolPageDataNode
	if search == "" {
		err = db.DB.Select(&dbResult, fmt.Sprintf(`
			select rocketpool_nodes.*, cnt.total_count
			from rocketpool_nodes
			left join (select count(*) from rocketpool_nodes) cnt(total_count) ON true
			order by %s %s
			limit $1
			offset $2`, orderBy, orderDir), length, start)
		if err != nil {
			logger.Errorf("error getting rocketpool-nodes from db: %v", err)
			http.Error(w, "Internal server error", 503)
			return
		}
	} else {
		err = db.DB.Select(&dbResult, fmt.Sprintf(`
			with matched_nodes as (
				select address from rocketpool_nodes where encode(address::bytea,'hex') like $3
			)
			select rocketpool_nodes.*, cnt.total_count
			from rocketpool_nodes
			inner join matched_nodes on matched_nodes.address = rocketpool_nodes.address
			left join (select count(*) from rocketpool_nodes) cnt(total_count) ON true
			order by %s %s
			limit $1
			offset $2`, orderBy, orderDir), length, start, search+"%")
		if err != nil {
			logger.Errorf("error getting rocketpool-nodes from db (with search: %v): %v", search, err)
			http.Error(w, "Internal server error", 503)
			return
		}
	}

	if len(dbResult) > 0 {
		recordsTotal = dbResult[0].TotalCount
		recordsFiltered = dbResult[0].TotalCount
	}

	tableData := make([][]interface{}, 0, len(dbResult))

	for _, row := range dbResult {
		entry := []interface{}{}
		entry = append(entry, utils.FormatEth1Address(row.Address))
		entry = append(entry, row.TimezoneLocation)
		entry = append(entry, row.RPLStake)
		entry = append(entry, row.MinRPLStake)
		entry = append(entry, row.MaxRPLStake)
		tableData = append(tableData, entry)
	}

	data := &types.DataTableResponse{
		Draw:            draw,
		RecordsTotal:    recordsTotal,
		RecordsFiltered: recordsFiltered,
		Data:            tableData,
	}

	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Errorf("error enconding json response for %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", 503)
		return
	}
}

func PoolsRocketpoolDataDAOProposals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()
	draw, err := strconv.ParseUint(q.Get("draw"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables data parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	start, err := strconv.ParseUint(q.Get("start"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables start parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	length, err := strconv.ParseUint(q.Get("length"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables length parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	if length > 100 {
		length = 100
	}
	search := strings.Replace(q.Get("search[value]"), "0x", "", -1)
	if len(search) > 128 {
		search = search[:128]
	}

	orderColumn := q.Get("order[0][column]")
	orderByMap := map[string]string{
		"0":  "id",
		"1":  "dao",
		"2":  "proposer",
		"3":  "message",
		"14": "is_executed",
		"15": "payload",
		"16": "state",
	}
	orderBy, exists := orderByMap[orderColumn]
	if !exists {
		orderBy = "id"
	}
	orderDir := q.Get("order[0][dir]")
	if orderDir != "desc" && orderDir != "asc" {
		orderDir = "desc"
	}

	recordsTotal := uint64(0)
	recordsFiltered := uint64(0)
	var dbResult []types.RocketpoolPageDataDAOProposal
	if search == "" {
		err = db.DB.Select(&dbResult, fmt.Sprintf(`
			select rocketpool_dao_proposals.*, cnt.total_count
			from rocketpool_dao_proposals
			left join (select count(*) from rocketpool_dao_proposals) cnt(total_count) ON true
			order by %s %s
			limit $1
			offset $2`, orderBy, orderDir), length, start)
		if err != nil {
			logger.Errorf("error getting rocketpool-proposals from db: %v", err)
			http.Error(w, "Internal server error", 503)
			return
		}
	} else {
		err = db.DB.Select(&dbResult, fmt.Sprintf(`
			with matched_proposals as (
				select id from rocketpool_dao_proposals where cast(id as text) like $3
				union select id from rocketpool_dao_proposals where dao like $5
				union select id from rocketpool_dao_proposals where message ilike $5
				union select id from rocketpool_dao_proposals where state = $3
				union select id from rocketpool_dao_proposals where encode(proposer_address::bytea,'hex') like $4
			)
			select 
				rocketpool_dao_proposals.*, 
				cnt.total_count
			from rocketpool_dao_proposals
			inner join matched_proposals on matched_proposals.id = rocketpool_dao_proposals.id
			left join (select count(*) from matched_proposals) cnt(total_count) ON true
			order by %s %s
			limit $1
			offset $2`, orderBy, orderDir), length, start, search, search+"%", "%"+search+"%")
		if err != nil {
			logger.Errorf("error getting rocketpool-proposals from db (with search: %v): %v", search, err)
			http.Error(w, "Internal server error", 503)
			return
		}
	}

	if len(dbResult) > 0 {
		recordsTotal = dbResult[0].TotalCount
		recordsFiltered = dbResult[0].TotalCount
	}

	tableData := make([][]interface{}, 0, len(dbResult))

	for _, row := range dbResult {
		entry := []interface{}{}
		entry = append(entry, row.ID)
		entry = append(entry, row.DAO)
		entry = append(entry, utils.FormatEth1Address(row.ProposerAddress))
		entry = append(entry, template.HTMLEscapeString(row.Message))
		entry = append(entry, utils.FormatTimestamp(row.CreatedTime.Unix()))
		entry = append(entry, utils.FormatTimestamp(row.StartTime.Unix()))
		entry = append(entry, utils.FormatTimestamp(row.EndTime.Unix()))
		entry = append(entry, utils.FormatTimestamp(row.ExpiryTime.Unix()))
		entry = append(entry, row.VotesRequired)
		entry = append(entry, row.VotesFor)
		entry = append(entry, row.VotesAgainst)
		entry = append(entry, row.MemberVoted)
		entry = append(entry, row.MemberSupported)
		entry = append(entry, row.IsCancelled)
		entry = append(entry, row.IsExecuted)
		if len(row.Payload) > 4 {
			entry = append(entry, fmt.Sprintf(`<span>%x…%x<span><i class="fa fa-copy text-muted ml-2 p-1" role="button" data-toggle="tooltip" title="Copy to clipboard" data-clipboard-text="%x"></i>`, row.Payload[:2], row.Payload[len(row.Payload)-2:], row.Payload))
			// entry = append(entry, fmt.Sprintf(`<span>%x…%x<span> <button class="btn btn-dark text-white btn-sm" type="button" data-toggle="tooltip" title="" data-clipboard-text="%x" data-original-title="Copy to clipboard"><i class="fa fa-copy"></i></button>`, row.Payload[:2], row.Payload[len(row.Payload)-2:], row.Payload))
			// entry = append(entry, fmt.Sprintf(`<span id="rocketpool-dao-proposal-payload-%v">%x…%x</span> <button></button>`, i, row.Payload[:2], row.Payload[len(row.Payload)-2:]))
		} else {
			entry = append(entry, fmt.Sprintf("%x", row.Payload))
		}

		entry = append(entry, row.State)
		tableData = append(tableData, entry)
	}

	data := &types.DataTableResponse{
		Draw:            draw,
		RecordsTotal:    recordsTotal,
		RecordsFiltered: recordsFiltered,
		Data:            tableData,
	}

	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Errorf("error enconding json response for %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", 503)
		return
	}
}

func PoolsRocketpoolDataDAOMembers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()
	draw, err := strconv.ParseUint(q.Get("draw"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables data parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	start, err := strconv.ParseUint(q.Get("start"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables start parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	length, err := strconv.ParseUint(q.Get("length"), 10, 64)
	if err != nil {
		logger.Errorf("error converting datatables length parameter from string to int: %v", err)
		http.Error(w, "Internal server error", 503)
		return
	}
	if length > 100 {
		length = 100
	}
	search := strings.Replace(q.Get("search[value]"), "0x", "", -1)
	if len(search) > 128 {
		search = search[:128]
	}

	orderColumn := q.Get("order[0][column]")
	orderByMap := map[string]string{
		"0": "address",
		"1": "id",
		"2": "url",
		"3": "joined_time",
		"4": "last_proposal_time",
		"5": "rpl_bond_amount",
		"6": "unbonded_validator_count",
	}
	orderBy, exists := orderByMap[orderColumn]
	if !exists {
		orderBy = "id"
	}
	orderDir := q.Get("order[0][dir]")
	if orderDir != "desc" && orderDir != "asc" {
		orderDir = "desc"
	}

	recordsTotal := uint64(0)
	recordsFiltered := uint64(0)
	var dbResult []types.RocketpoolPageDataDAOMember
	if search == "" {
		err = db.DB.Select(&dbResult, fmt.Sprintf(`
			select rocketpool_dao_members.*, cnt.total_count
			from rocketpool_dao_members
			left join (select count(*) from rocketpool_dao_members) cnt(total_count) ON true
			order by %s %s
			limit $1
			offset $2`, orderBy, orderDir), length, start)
		if err != nil {
			logger.Errorf("error getting rocketpool-members from db: %v", err)
			http.Error(w, "Internal server error", 503)
			return
		}
	} else {
		err = db.DB.Select(&dbResult, fmt.Sprintf(`
			with matched_members as (
				select address from rocketpool_dao_members where encode(address::bytea,'hex') like $3
				union select address from rocketpool_dao_members where id ilike $4
				union select address from rocketpool_dao_members where url ilike $4
			)
			select rocketpool_dao_members.*, cnt.total_count
			from rocketpool_dao_members
			inner join matched_members on matched_members.address = rocketpool_dao_members.address
			left join (select count(*) from matched_members) cnt(total_count) ON true
			order by %s %s
			limit $1
			offset $2`, orderBy, orderDir), length, start, search+"%", "%"+search+"%")
		if err != nil {
			logger.Errorf("error getting rocketpool-members from db (with search: %v): %v", search, err)
			http.Error(w, "Internal server error", 503)
			return
		}
	}

	if len(dbResult) > 0 {
		recordsTotal = dbResult[0].TotalCount
		recordsFiltered = dbResult[0].TotalCount
	}

	tableData := make([][]interface{}, 0, len(dbResult))

	for _, row := range dbResult {
		entry := []interface{}{}
		entry = append(entry, utils.FormatEth1Address(row.Address))
		entry = append(entry, row.ID)
		entry = append(entry, row.URL)
		entry = append(entry, utils.FormatTimestamp(row.JoinedTime.Unix()))
		entry = append(entry, utils.FormatTimestamp(row.LastProposalTime.Unix()))
		entry = append(entry, row.RPLBondAmount)
		entry = append(entry, row.UnbondedValidatorCount)
		tableData = append(tableData, entry)
	}

	data := &types.DataTableResponse{
		Draw:            draw,
		RecordsTotal:    recordsTotal,
		RecordsFiltered: recordsFiltered,
		Data:            tableData,
	}

	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Errorf("error enconding json response for %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", 503)
		return
	}
}