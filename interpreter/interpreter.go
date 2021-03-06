package interpreter

import (
	"GoGraphDb/conf"
	"GoGraphDb/log"
	"GoGraphDb/transaction"
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"strings"
)

var (
	TransactionCounter = make(chan int64, 10)
)

type Interpreter struct {
	transaction *transaction.Transaction
}

func (i *Interpreter) ExeDmlFile(f *os.File) error{
	reader := bufio.NewReader(f)
	for {
		row, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.CtxError(context.Background(), "execute dml file wrong, read data file error: %+v", err)
			}
			break
		}
		//去除换行符
		row = strings.ReplaceAll(row, "\r", "")
		row = strings.ReplaceAll(row, "\n", "")
		if len(row) == 0 {
			log.CtxWarn(context.Background(), "dml file has an empty line")
			continue
		}
		units := strings.Split(row, conf.Splitor)
		//处理事务的开始与结束逻辑
		if len(units) == 1 {
			err := i.ChangeStatus(units[0])
			if err != nil {
				log.CtxError(context.Background(), "interpreter change status error: %+v", err)
				panic(err)
			}
			continue
		}
		//处理三元组语法
		if len(units) != 3 {
			log.CtxError(context.Background(), "interpreter sentence error, content: %+v", units)
			continue
		}else{
			i.ExecuteSentence(units[0], units[1], units[2])
		}
	}
	return nil
}

func (i *Interpreter) ChangeStatus(command string) error{
	switch command {
	case conf.InterpreterCommand_StartTransaction:
		{
			//并发控制 数据落盘时避免新的事务开始
			<-transaction.StopTheWorld
			if i.transaction != nil {
				err := errors.New("Transation MultiStart")
				log.CtxError(context.Background(), err.Error())
				return err
			}
			i.transaction = transaction.NewTransaction()
			transaction.TransactionGetter[i.transaction.Version] = i.transaction
			log.CtxInfo(context.Background(),"start Transaction, version: %+v", i.transaction.Version)
			//事务计数
			TransactionCounter <- 0
		}
	case conf.InterpreterCommand_StartReadOnlyTransaction:
		{
			//并发控制 数据落盘时避免新的事务开始
			<-transaction.StopTheWorld
			if i.transaction != nil {
				err := errors.New("Transation MultiStart")
				log.CtxError(context.Background(), err.Error())
				return err
			}
			i.transaction = transaction.NewReadOnlyTransaction()
			log.CtxInfo(context.Background(),"start read_only Transaction, version: %+v", i.transaction.Version)
		}
	case conf.InterpreterCommand_EndTransaction:
		{
			if i.transaction == nil {
				err := errors.New("No Executing Transation")
				log.CtxError(context.Background(), err.Error())
				return err
			}
			err := i.transaction.End()
			if err != nil {
				log.CtxError(context.Background(), err.Error())
				i.transaction.RollBack()
				return err
			}
			log.CtxInfo(context.Background(),"end Transaction, version: %+v", i.transaction.Version)
			i.transaction = nil
		}
	default:
		{
			err := errors.New("Interpreter command error, command: " + command)
			log.CtxError(context.Background(), err.Error())
			return err
		}
	}
	return nil
}

