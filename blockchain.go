package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Transaction struct {
	UserId                   string
	Day                      *big.Int
	Month                    *big.Int
	Year                     *big.Int
	AssetId                  string
	TotalDuration            *big.Int
	TotalRewardsConsumer     *big.Int
	TotalRewardsContentOwner *big.Int
}

func addToBlockchain(jobs []JobDataRow) error {
	client, err := ethclient.Dial("https://base-sepolia-rpc.publicnode.com")
	if err != nil {
		log.Printf("Erro ao conectar ao cliente Ethereum: %v", err)
		return err
	}

	key := "0x675a916e0fa4bfa9435cafb158173059bc3057bbabd11016ede6f3b7d37add3b"
	if strings.HasPrefix(key, "0x") {
		key = key[2:]
	}
	privateKey, err := crypto.HexToECDSA(key)
	if err != nil {
		log.Printf("Erro ao converter chave privada: %v", err)
		return err
	}

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return err
	}

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return err
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return err
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // Sem valor transferido
	auth.GasLimit = uint64(5000000) // Aumenta o GasLimit
	auth.GasPrice = gasPrice

	contractAddress := common.HexToAddress("0x54953C813543AAB514E8B91C02680aD530b6ccA6")
	parsedABI, err := abi.JSON(strings.NewReader(ABI))
	if err != nil {
		log.Printf("Erro ao analisar o ABI: %v", err)
		return err
	}

	contract := bind.NewBoundContract(contractAddress, parsedABI, client, client, client)

	if err != nil {
		log.Printf("Erro ao estimar o gás: %v", err)
		return err
	}

	transactions := make([]Transaction, len(jobs))
	for i, job := range jobs {
		assetID := ""
		if job.AssetID.Valid {
			assetID = job.AssetID.StringVal
		}

		totalRewardsConsumerWei := ToWei(job.TotalRewardsConsumer)
		totalRewardsContentOwnerWei := ToWei(job.TotalRewardsContentOwner)

		transactions[i] = Transaction{
			UserId:                   job.UserID,
			Day:                      big.NewInt(int64(job.CreatedAtDay.Day())),
			Month:                    big.NewInt(int64(job.CreatedAtDay.Month())),
			Year:                     big.NewInt(int64(job.CreatedAtDay.Year())),
			AssetId:                  assetID,
			TotalDuration:            big.NewInt(job.TotalDuration),
			TotalRewardsConsumer:     totalRewardsConsumerWei,
			TotalRewardsContentOwner: totalRewardsContentOwnerWei,
		}
	}

	callData, err := parsedABI.Pack("batchInsertRecords", transactions)
	if err != nil {
		log.Printf("Erro ao empacotar os dados da transação: %v", err)
		return err
	}

	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		To:   &contractAddress,
		Data: callData,
	})

	auth.GasLimit = gasLimit + gasLimit/5 // Increase gas limit by 20%

	tx, err := contract.Transact(auth, "batchInsertRecords", transactions)
	if err != nil {
		log.Printf("Erro ao enviar transação: %v", err)
		return err
	}

	if tx == nil {
		log.Printf("Transação retornada é nula")
		return fmt.Errorf("transação retornada é nula")
	}

	receipt, err := waitForConfirmation(client, tx.Hash())
	if err != nil {
		log.Printf("Erro ao aguardar a confirmação da transação. Hash: %s, Erro: %v", tx.Hash().Hex(), err)
		return err
	}

	if receipt.Status == 1 {
		log.Printf("Transação confirmada com sucesso! Hash: %s", tx.Hash().Hex())
	} else {
		fmt.Printf("Transação falhou com status: %d, erro: %v", receipt.Status, err)
		return fmt.Errorf("transação falhou com status: %d", receipt.Status)
	}

	return nil
}

func waitForConfirmation(client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	for {
		receipt, err := client.TransactionReceipt(context.Background(), txHash)
		if err == ethereum.NotFound {
			time.Sleep(time.Second * 2)
			continue
		} else if err != nil {
			return nil, err
		}

		return receipt, nil
	}
}

func ToWei(ethValue float64) *big.Int {
	weiValue := new(big.Float).Mul(big.NewFloat(ethValue), big.NewFloat(1e18))
	weiBigInt := new(big.Int)
	weiValue.Int(weiBigInt)
	return weiBigInt
}