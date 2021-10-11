package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/leekchan/accounting"
	"github.com/urfave/cli/v2"
)

func main() {
	var dateFormat = "2006-01-02"
	var firstMonthStart string
	var secondMonthStart string
	var costMetric string
	var serviceFilter string
	var sortColumn string
	var sortOrder string
	var output string
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
			&cli.StringSliceFlag{
				Name:  "tag",
				Usage: "Tag value to filter results (app=web, env=prod, etc.)",
			},
			&cli.StringFlag{
				Name:        "sort",
				Value:       "name",
				Usage:       "Column to sort results on (name, start, end, delta, deltapercent)",
				Destination: &sortColumn,
			},
			&cli.StringFlag{
				Name:        "sort-order",
				Value:       "asc",
				Usage:       "Order to sort in (asc or desc)",
				Destination: &sortOrder,
			},
			&cli.StringFlag{
				Name:        "output",
				Value:       "table",
				Usage:       "Output format (supported formats: table, csv)",
				Destination: &output,
			},
		},
		Action: func(c *cli.Context) error {
			var ac *accounting.Accounting
			ac = accounting.DefaultAccounting("$", 2)
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

			tagFilters := c.StringSlice("tag")

			firstResultsCosts := GetCosts(svc, firstMonthStart, firstMonthEnd, costMetric, grouping, serviceFilter, tagFilters)
			secondResultsCosts := GetCosts(svc, secondMonthStart, secondMonthEnd, costMetric, grouping, serviceFilter, tagFilters)
			allServiceNames := ExtractAllServiceNames(firstResultsCosts, secondResultsCosts)

			type ServiceCosts struct {
				serviceName  string
				amount       float64
				secondAmount float64
				delta        float64
				deltaPercent float64
			}

			var finalResultsCosts []ServiceCosts
			finalResultsCosts = make([]ServiceCosts, 0, len(allServiceNames))
			var totalAmount = 0.0
			var totalSecondAmount = 0.0
			var totalDelta = 0.0
			var totalDeltaPercent = 0.0
			for _, service := range allServiceNames {
				amount, _ := firstResultsCosts[service]
				secondAmount, _ := secondResultsCosts[service]
				secondAmount *= multiplier
				delta := secondAmount - amount
				deltaPercent := 0.0
				if delta != 0 {
					deltaPercent = delta / amount * 100
				}

				finalResultsCosts = append(finalResultsCosts, ServiceCosts{
					service,
					amount,
					secondAmount,
					delta,
					deltaPercent,
				})

				totalAmount += amount
				totalSecondAmount += secondAmount
				totalDelta += delta
			}

			// Sort results
			sort.Slice(finalResultsCosts, func(i, j int) bool {
				var retVal bool
				switch sortColumn {
				case "start":
					retVal = finalResultsCosts[i].amount < finalResultsCosts[j].amount
				case "end":
					retVal = finalResultsCosts[i].secondAmount < finalResultsCosts[j].secondAmount
				case "delta":
					retVal = finalResultsCosts[i].delta < finalResultsCosts[j].delta
				case "deltapercent":
					retVal = finalResultsCosts[i].deltaPercent < finalResultsCosts[j].deltaPercent
				default: // default to service name
					retVal = strings.ToLower(finalResultsCosts[i].serviceName) < strings.ToLower(finalResultsCosts[j].serviceName)
				}
				if sortOrder == "desc" {
					retVal = !retVal
				}
				return retVal
			})

			// Render Table
			tw := table.NewWriter()
			var secondMonthHeader = secondMonthStart
			if isProjection {
				secondMonthHeader += " (projection)"
			}

			// Remove commas to make output compatible for CSVs
			if output == "csv" {
				ac.SetThousandSeparator("")
			}

			tw.AppendHeader(table.Row{"Service", firstMonthStart, secondMonthHeader, "Delta", "Delta Percent"})
			for _, serviceCosts := range finalResultsCosts {
				tw.AppendRow(table.Row{serviceCosts.serviceName, ac.FormatMoney(serviceCosts.amount), ac.FormatMoney(serviceCosts.secondAmount), ac.FormatMoney(serviceCosts.delta), fmt.Sprintf("%s%%%%", accounting.FormatNumber(serviceCosts.deltaPercent, 1, "", "."))})
			}

			totalDeltaPercent = totalDelta / totalAmount * 100

			tw.AppendFooter(table.Row{"Total", ac.FormatMoney(totalAmount), ac.FormatMoney(totalSecondAmount), ac.FormatMoney(totalDelta), fmt.Sprintf("%s%%%%", accounting.FormatNumber(totalDeltaPercent, 1, "", "."))})

			tw.SetColumnConfigs([]table.ColumnConfig{
				{Name: firstMonthStart, Align: text.AlignRight, AlignFooter: text.AlignRight},
				{Name: secondMonthHeader, Align: text.AlignRight, AlignFooter: text.AlignRight},
				{Name: "Delta", Align: text.AlignRight, AlignFooter: text.AlignRight},
				{Name: "Delta Percent", Align: text.AlignRight, AlignFooter: text.AlignRight},
			})

			switch output {
			case "csv":
				fmt.Printf(tw.RenderCSV())
			default:
				fmt.Printf("\n")
				fmt.Printf(tw.Render())
				fmt.Printf("\n")
			}

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

func GetCosts(svc *costexplorer.CostExplorer, start string, end string, costmetric string, grouping string, serviceFilter string, tagFilters []string) map[string]float64 {
	// Assemble filters
	var filters []*costexplorer.Expression
	if len(tagFilters) > 0 {
		for _, tagFilter := range tagFilters {
			tagParts := strings.SplitN(tagFilter, "=", 2)
			if len(tagParts) == 2 {
				filters = append(filters, GetTagExpression(tagParts[0], tagParts[1]))
			}
		}
	}
	if serviceFilter != "" {
		filters = append(filters, GetDimensionExpression("SERVICE", serviceFilter))
	}
	var filter *costexplorer.Expression
	if len(filters) > 1 {
		filter = &costexplorer.Expression{
			And: filters,
		}
	} else if len(filters) == 1 {
		filter = filters[0]
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
				rawAmount, err := strconv.ParseFloat(*metrics.Amount, 64)

				// Round numbers immediately so we don't have to worry about weird delta math on values like $0.0000000061
				amount := math.Round(rawAmount*100) / 100
				if err != nil {
					fmt.Println(err)
				}
				resultsCosts[*groups.Keys[0]] = amount
			}
		}
	}
	return resultsCosts
}

func GetTagExpression(tag string, value string) *costexplorer.Expression {
	return &costexplorer.Expression{
		Tags: &costexplorer.TagValues{
			Key:    aws.String(tag),
			Values: aws.StringSlice([]string{value}),
		},
	}
}

func GetDimensionExpression(dimension string, value string) *costexplorer.Expression {
	return &costexplorer.Expression{
		Dimensions: &costexplorer.DimensionValues{
			Key:    aws.String(dimension),
			Values: aws.StringSlice([]string{value}),
		},
	}
}
