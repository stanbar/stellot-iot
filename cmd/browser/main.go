package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stellar/go/keypair"
	"github.com/stellot/stellot-iot/pkg/aggregator"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/functions"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"github.com/stellot/stellot-iot/pkg/utils"
)

func main() {
	dbpool, err := pgxpool.Connect(context.Background(), helpers.DatabaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	// for _, aggregator := range aggregator.Aggregators[3:] {
	// 	res := getValuesFromPeroid(dbpool, functions.AvgAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, 10000000000)
	// 	log.Println("avg", res)
	// }

	// for _, aggregator := range aggregator.Aggregators[3:] {
	// 	res := getValuesFromPeroid(dbpool, functions.MinAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, 10000000000)
	// 	log.Println("min", res)
	// }

	// for _, aggregator := range aggregator.Aggregators[3:] {
	// 	res := getValuesFromPeroid(dbpool, functions.MaxAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, 10000000000)
	// 	log.Println("max", res)
	// }

	// for _, aggregator := range aggregator.Aggregators {
	// 	res := getValuesPredicate(dbpool, functions.MinAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, ">", 500)
	// 	log.Println("min > 500", res)
	// }

	// for _, aggregator := range aggregator.Aggregators {
	// 	res := getValuesPredicate(dbpool, functions.MaxAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, "<", 700)
	// 	log.Println("max < 700", res)
	// }

	// getValuesPredicateTx(dbpool, usecases.HUMD, helpers.DevicesKeypairs[0], "<", 700)

	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.TEMP, functions.AVG, 2, aggregator.SIX_HOURS)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.FIVE_SECS)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.THIRTY_SECS)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.ONE_MIN)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.FIVE_MINS)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.THIRTY_MINS)

	// getValuesPredicate(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.FIVE_SECS), "<", 700)
	// getValuesPredicateTx(dbpool, usecases.HUMD, helpers.DevicesKeypairs[0], "<", 700)
	values := getValuesFromPeroidTx(dbpool, helpers.DevicesKeypairs[0].Address(), 100000000000)
	log.Println(len(values))
	values = getValuesFromPeroid(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.FIVE_SECS), 100000000000)
	log.Println(len(values))
	values = getValuesFromPeroid(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.ONE_MIN), 100000000000)
	log.Println(len(values))
	values = getValuesFromPeroid(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.ONE_HOUR), 100000000000)
	log.Println(len(values))
	values = getValuesFromPeroid(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.ONE_DAY), 100000000000)
	log.Println(len(values))

}

func countTxs(dbpool *pgxpool.Pool, sensorAddress string, usecase usecases.PhysicsType, function functions.FunctionType, lastTxs int, timeInterval aggregator.TimeInterval) {
	defer utils.Duration(utils.Track("countTxs"))
	start := time.Now()

	from := aggregator.ByTimeInterval(timeInterval).Keypair.Address()
	to := sensorAddress
	assetCode := function.Asset().GetCode()
	offset := lastTxs

	log.Println("from: ", from, " to: ", to, " assetCode: ", assetCode, " offset: ", offset)

	row := dbpool.QueryRow(context.Background(), `
  SELECT transaction_id FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE source_account = $1
    AND type = 1
    AND details->>'from' = $2
    AND details->>'to' = $3
    AND details->>'asset_code' = $4
  ORDER BY account_sequence DESC
  LIMIT 1
  OFFSET $5;
  `, from, from, to, assetCode, offset)
	log.Println("executed sql", time.Since(start))

	var txId int64
	err := row.Scan(&txId)
	if err != nil {
		panic(err)
	}
	log.Println("txId: ", txId)

	rows, err := dbpool.Query(context.Background(), `
  SELECT details->>'bump_to' FROM history_operations ops
  WHERE transaction_id = $1
    AND details->>'bump_to' IS NOT NULL
  ORDER BY application_order
  `, txId)
	log.Println("executed sql", time.Since(start))

	var (
		fromSeq string
		toSeq   string
	)
	rows.Next()
	err = rows.Scan(&fromSeq)
	if err != nil {
		panic(err)
	}

	rows.Next()
	err = rows.Scan(&toSeq)
	if err != nil {
		panic(err)
	}

	log.Printf("fromSeq: %s toSeq: %s", fromSeq, toSeq)

	row = dbpool.QueryRow(context.Background(), `
  SELECT count(*) FROM history_transactions txs
  WHERE txs.account = $1
  AND txs.account_sequence > $2
  AND txs.account_sequence <= $3
  GROUP BY txs.account;
  `, to, fromSeq, toSeq)
	log.Println("executed sql", time.Since(start))

	var count int64
	log.Println("scanning")
	err = row.Scan(&count)
	log.Println("after scanning")
	if err != nil {
		log.Println("Error")
		log.Fatal(err)
		panic(err)
	}

	log.Println("count: ", count)
}

func getValuesPredicateTx(dbpool *pgxpool.Pool, usecase usecases.PhysicsType, sensor *keypair.Full, operation string, predicate int) []int {
	defer utils.Duration(utils.Track("getValuesPredicateTx"))
	from := sensor.Address()
	to := helpers.BatchKeypair.Address()
	assetCode := usecase.Asset().GetCode()

	log.Println("from: ", from, " to: ", to, " assetCode: ", assetCode)

	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  `, from, to, assetCode)
	log.Println("execute sql", time.Since(start))

	if err != nil {
		log.Fatalln(err)
		panic(err)
	}

	start = time.Now()
	values := []int{}
	for _, v := range parseValues(rows, sensor, to) {
		switch operation {
		case ">":
			if v > predicate {
				values = append(values, v)
			}
		case ">=":
			if v >= predicate {
				values = append(values, v)
			}
		case "<":
			if v < predicate {
				values = append(values, v)
			}
		case "<=":
			if v <= predicate {
				values = append(values, v)
			}
		}
	}
	log.Printf("parsed %d values in %s", len(values), time.Since(start))
	return values
}

func getValuesFromPeroid(dbpool *pgxpool.Pool, function functions.FunctionType, sensorAddress string, aggregator aggregator.Aggregator, lastTxs int) []int {
	defer utils.Duration(utils.Track("getValuesFromPeroid"))
	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  LIMIT $4;
  `, aggregator.Keypair.Address(), sensorAddress, function.Asset().GetCode(), lastTxs)

	elapsed := time.Since(start)
	log.Println("execute sql", elapsed)

	if err != nil {
		panic(err)
	}
	return parseValues(rows, aggregator.Keypair, sensorAddress)
}

func getValuesFromPeroidTx(dbpool *pgxpool.Pool, sensorAddress string, lastTxs int) []int {
	defer utils.Duration(utils.Track("getValuesFromPeroidTx"))
	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
  ORDER BY account_sequence DESC
  LIMIT $3;
  `, sensorAddress, helpers.BatchKeypair.Address(), lastTxs)

	elapsed := time.Since(start)
	log.Println("execute sql", elapsed)

	if err != nil {
		panic(err)
	}
	return parseValues(rows, helpers.BatchKeypair, sensorAddress)
}

func getValuesPredicate(dbpool *pgxpool.Pool, function functions.FunctionType, sensorAddress string, aggregator aggregator.Aggregator, operation string, predicate int) []int {
	defer utils.Duration(utils.Track("getValuesPredicate"))
	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  `, aggregator.Keypair.Address(), sensorAddress, function.Asset().GetCode())

	log.Println("execute sql", time.Since(start))

	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	values := []int{}
	for _, v := range parseValues(rows, aggregator.Keypair, sensorAddress) {
		switch operation {
		case ">":
			if v > predicate {
				values = append(values, v)
			}
		case ">=":
			if v >= predicate {
				values = append(values, v)
			}
		case "<":
			if v < predicate {
				values = append(values, v)
			}
		case "<=":
			if v <= predicate {
				values = append(values, v)
			}
		}
	}
	return values
}

func getFromLastNFiveSecondsIntervals(dbpool *pgxpool.Pool, function functions.FunctionType, timeKeypair *keypair.Full, sensorAddres string, lastBlocks int64) []int {
	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  LIMIT $4;
`, timeKeypair.Address(), sensorAddres, function.Asset().GetCode(), lastBlocks)

	elapsed := time.Since(start)
	log.Println("execute sql", elapsed)

	if err != nil {
		log.Fatal(err)
	}
	return parseValues(rows, timeKeypair, sensorAddres)
}

func parseValues(rows pgx.Rows, sender *keypair.Full, receiver string) []int {
	values := make([]int, 0)

	for rows.Next() {
		var (
			memo       string
			accountSeq int64
		)
		err := rows.Scan(&memo, &accountSeq)
		if err != nil {
			log.Fatal(err)
		}
		intValue := decodeMemo(memo, accountSeq, sender, receiver)
		values = append(values, int(intValue))
	}
	return values
}

func decodeMemo(memo string, accountSeq int64, timeKeypair *keypair.Full, sensorAddress string) int64 {
	out := make([]byte, base64.StdEncoding.DecodedLen(len(memo)))
	length, err := base64.StdEncoding.Decode(out, []byte(memo))
	if err != nil {
		log.Fatal(err)
	}

	var memoBytes [32]byte
	copy(memoBytes[:], out[:length])

	decrypted, err := crypto.EncryptToMemo(accountSeq, timeKeypair, sensorAddress, memoBytes)
	decryptedValue := strings.Trim(string(decrypted[:]), string(rune(0)))
	intValue, _ := strconv.ParseInt(decryptedValue, 10, 32)
	return intValue
}
