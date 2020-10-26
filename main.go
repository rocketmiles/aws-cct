package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/leekchan/accounting"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

func main() {
	var dateFormat = "2006-01-02"
	var firstMonthStart string
	var secondMonthStart string
	var costMetric string
	var serviceFilter string
	currentDate := time.Now()
	thisMonthFirst := time.Date(currentDate.Year(), currentDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	previousMonthFirst := thisMonthFirst.AddDate(0, -1, 0)
	nextMonthFirst := thisMonthFirst.AddDate(0, 1, 0)
	lastDayOfThisMonth := nextMonthFirst.AddDate(0, 0, -1).Day()
	app := &cli.App{
		Name:  "aws-cct",
		Usage: "AWS Cost Comparison Tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "start",
				Value:       previousMonthFirst.Format(dateFormat),
				Usage:       "First month to compare (2020-01-01)",
				Destination: &firstMonthStart,
			},
			&cli.StringFlag{
				Name:        "end",
				Value:       thisMonthFirst.Format(dateFormat),
				Usage:       "Second month to compare (2020-02-01)",
				Destination: &secondMonthStart,
			},
			&cli.StringFlag{
				Name:        "cost-metric",
				Value:       "NetAmortizedCost",
				Usage:       "Cost Metric to compare (NetAmortizedCost, UnblendedCost, etc.)",
				Destination: &costMetric,
			},
			&cli.StringFlag{
				Name:        "service",
				Value:       "",
				Usage:       "Define a service to dig into",
				Destination: &serviceFilter,
			},
		},
		Action: func(c *cli.Context) error {
			ac := accounting.Accounting{Symbol: "$", Precision: 2}
			sess, _ := session.NewSession()
			svc := costexplorer.New(sess)

			start, _ := time.Parse(dateFormat, firstMonthStart)
			firstMonthEnd := start.AddDate(0, 1, 0).Format(dateFormat)
			end, _ := time.Parse(dateFormat, secondMonthStart)
			isProjection := false
			multiplier := 1.0
			var secondMonthEnd string
			if currentDate.Month() == end.Month() {
				isProjection = true
				// Go one day before current date for accuracy
				secondMonthEndDate := currentDate.AddDate(0, 0, -1)
				// This is the remaining projection multiplier
				multiplier = float64(lastDayOfThisMonth) / float64(secondMonthEndDate.Day())
				secondMonthEnd = secondMonthEndDate.Format(dateFormat)
			} else {
				secondMonthEnd = end.AddDate(0, 1, 0).Format(dateFormat)
			}

			var grouping = "SERVICE"

			if serviceFilter != "" {
				grouping = "USAGE_TYPE"
			}

			firstResultsCosts := GetCosts(svc, firstMonthStart, firstMonthEnd, costMetric, grouping, serviceFilter)
			secondResultsCosts := GetCosts(svc, secondMonthStart, secondMonthEnd, costMetric, grouping, serviceFilter)
			allServiceNames := ExtractAllServiceNames(firstResultsCosts, secondResultsCosts)

			var finalResultsCosts map[string][]float64
			finalResultsCosts = make(map[string][]float64)
			var totalAmount = 0.0
			var totalSecondAmount = 0.0
			var totalDelta = 0.0
			for _, service := range allServiceNames {
				amount, _ := firstResultsCosts[service]
				secondAmount, _ := secondResultsCosts[service]
				secondAmount *= multiplier
				delta := secondAmount - amount
				finalResultsCosts[service] = []float64{
					amount,
					secondAmount,
					delta,
				}
				totalAmount += amount
				totalSecondAmount += secondAmount
				totalDelta += delta
			}
			// Render Table
			tw := table.NewWriter()
			var secondMonthHeader = secondMonthStart
			if isProjection {
				secondMonthHeader += " (projection)"
			}
			tw.AppendHeader(table.Row{"Service", firstMonthStart, secondMonthHeader, "Delta"})
			for service, amounts := range finalResultsCosts {
				tw.AppendRow(table.Row{service, ac.FormatMoney(amounts[0]), ac.FormatMoney(amounts[1]), ac.FormatMoney(amounts[2])})
			}
			tw.AppendFooter(table.Row{"Total", ac.FormatMoney(totalAmount), ac.FormatMoney(totalSecondAmount), ac.FormatMoney(totalDelta)})
			fmt.Printf("\n")
			fmt.Printf(tw.Render())
			fmt.Printf("\n")
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func ExtractAllServiceNames(firstResultsCosts map[string]float64, secondResultsCosts map[string]float64) []string {
	// Create the combined array
	var allServiceNames []string
	for serviceName, _ := range firstResultsCosts {
		allServiceNames = append(allServiceNames, serviceName)
	}
	for serviceName, _ := range secondResultsCosts {
		// Only get unique service names from second set
		if _, ok := firstResultsCosts[serviceName]; !ok {
			allServiceNames = append(allServiceNames, serviceName)
		}
	}
	sort.Strings(allServiceNames)
	return allServiceNames
}

func GetCosts(svc *costexplorer.CostExplorer, start string, end string, costmetric string, grouping string, serviceFilter string) map[string]float64 {
	var filter *costexplorer.Expression
	if serviceFilter != "" {
		filter = &costexplorer.Expression{
			Dimensions: &costexplorer.DimensionValues{
				Key:    aws.String("SERVICE"),
				Values: aws.StringSlice([]string{serviceFilter}),
			},
		}
	}
	costInput := &costexplorer.GetCostAndUsageInput{
		Filter:      filter,
		Granularity: aws.String("MONTHLY"),
		GroupBy: []*costexplorer.GroupDefinition{
			{
				Type: aws.String("DIMENSION"),
				Key:  aws.String(grouping),
			},
		},
		Metrics: []*string{
			aws.String(costmetric),
		},
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
	}

	req, resp := svc.GetCostAndUsageRequest(costInput)

	err := req.Send()
	if err != nil {
		fmt.Println(err)
	}
	var resultsCosts map[string]float64
	resultsCosts = make(map[string]float64)

	for _, results := range resp.ResultsByTime {
		for _, groups := range results.Groups {
			for _, metrics := range groups.Metrics {
				amount, err := strconv.ParseFloat(*metrics.Amount, 64)
				if err != nil {
					fmt.Println(err)
				}
				resultsCosts[*groups.Keys[0]] = amount
			}
		}
	}
	return resultsCosts
}
