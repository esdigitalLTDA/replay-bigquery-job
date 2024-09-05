package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

type JobDataRow struct {
	JobID                    string              `bigquery:"JOB_ID"`
	ChunkID                  float64             `bigquery:"CHUNK_ID"`
	UserID                   string              `bigquery:"userId"`
	AssetID                  bigquery.NullString `bigquery:"assetId"`
	TotalDuration            int64               `bigquery:"totalDuration"`
	TotalRewardsConsumer     float64             `bigquery:"totalRewardsConsumer"`
	TotalRewardsContentOwner float64             `bigquery:"totalRewardsContentOwner"`
	CreatedAtDay             time.Time           `bigquery:"createdAtDay"`
	Status                   bigquery.NullString `bigquery:"status"`
}

// Função para processar os jobs vindos do BigQuery
func processJobs(datetime time.Time, secretName string) error {
	client, err := GetBigQueryClient(secretName)
	if err != nil {
		log.Println("Falha ao criar cliente do BigQuery: ", err)
		return err
	}
	defer client.Close()

	dMinus1 := datetime.AddDate(0, 0, -1).Format("2006-01-02")
	queryStr := fmt.Sprintf(`
		SELECT
			CHUNK_ID,
			JOB_ID,
			assetId,
			createdAtDay,
			totalDuration,
			totalRewardsConsumer,
			totalRewardsContentOwner,
			userId,
			status
		FROM
			replay-353318.replayAnalytics.table_blockchain_chunked_data_of_the_day_and_asset
		WHERE
			createdAtDay = '%s' AND
		    status IS NULL
	`, dMinus1)

	query := client.Query(queryStr)

	rows, err := query.Read(context.Background())
	if err != nil {
		log.Println("Falha ao executar query: ", err)
		return err
	}

	var jobs []JobDataRow
	var count int

	for {
		var row JobDataRow
		err := rows.Next(&row)
		if errors.Is(err, iterator.Done) {
			if count == 0 {
				log.Println("A query retornou um conjunto de resultados vazio.")
			} else {
				log.Printf("Total de jobs lidos: %d", count)
			}
			break
		}
		if err != nil {
			log.Println("Falha ao ler resultados: ", err)
			return err
		}

		jobs = append(jobs, row)
		count++
	}

	err = addToBlockchain(jobs)

	if err != nil {
		log.Printf("Erro ao processar todos os jobs: %v", err)
		return err
	}

	return nil
}
