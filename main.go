package main
import ( 
		lineblocs "bitbucket.org/infinitet3ch/lineblocs-go-helpers"
		"strconv"
		"database/sql"
		"time"
		"fmt"
)
var db* sql.DB;

func inTimeSpan(start, end, check time.Time) bool {
    return check.After(start) && check.Before(end)
}

type Call struct {
	Duration int
	SipStatus string	
	Status string	
}
type MetricsData struct {
	ProviderId string
	CallDuration []int
	CallsAnswered int
	CallsTotal int
	SIPFailures int
	AvgCallDuration int
	AvgAnswerRate int
	FailureResponsePct int
}

func sum(array []int) int {  
 result := 0  
 for _, v := range array {  
  result += v  
 }  
 return result  
}  
func main() {
	var err error
	db, err =lineblocs.CreateDBConn()
	var metrics map[string]*MetricsData
	if err != nil {
		panic(err)
	}
	// get call records from last minute

	metrics=make( map[string]*MetricsData)
	duration:=time.Duration(time.Second*60)
	start:=time.Now().Local()
	deadline:=time.Now().Local().Add(duration)

	for ;; {
		now:=time.Now().Local()
		results, err := db.Query("SELECT `sip_status`, `sip_msg`, `status`, `duration`, `provider_id`, `created_at`, `updated_at` FROM calls WHERE metrics_processed=0");
		defer results.Close()
		if err != nil {
			fmt.Println("error: " + err.Error())
			fmt.Println("trying again in 30s")
			time.Sleep(time.Duration(time.Second * 30))
			continue
		}

		for results.Next() {
			var sipStatus int
			var sipMsg string
			var status string
			var duration int
			var providerId int
			var createdAt time.Time
			var updatedAt time.Time
			results.Scan(
				&sipStatus,
				&sipMsg,
				&status,
				&duration,
				&providerId,
				&createdAt,
				&updatedAt)

			provider := strconv.Itoa( providerId )

			metric, ok := metrics[provider]
			if !ok {
				// add metric
				metric= &MetricsData{ 
					CallDuration: make([]int, 0),
					CallsTotal: 0,
					CallsAnswered: 0,
					SIPFailures: 0, }
				metrics[provider] = metric
			}

			if inTimeSpan( start, deadline, createdAt ) {
				metric.CallsTotal = metric.CallsTotal + 1
				if status == "completed" {
					metric.CallsAnswered = metric.CallsAnswered + 1
				}
				if sipStatus >= 400 && sipStatus <= 499 {
					metric.SIPFailures= metric.SIPFailures + 1
				}
				if sipStatus >= 500 && sipStatus <= 599 {
					metric.SIPFailures= metric.SIPFailures + 1
				}

				metric.CallDuration = append( metric.CallDuration, duration )

				// calculate metrics
				sumOfDuration := sum( metric.CallDuration )
				metric.AvgCallDuration = sumOfDuration / len( metric.CallDuration )
				metric.AvgAnswerRate = ( metric.CallsAnswered / metric.CallsTotal ) * 100
				metric.FailureResponsePct = ( metric.SIPFailures / metric.CallsTotal ) * 100

			}

		}
    	if deadline.Before(now) {
			// set new times
			start=time.Now().Local()
			deadline=time.Now().Local().Add(duration)

			// write stats
			for provider,metric := range metrics {
				now:=time.Now().Local()
				// store the data
				stmt, err := db.Prepare("INSERT INTO sip_providers_metrics (`avg_answer_rate`, `avg_call_duration`, `failure_response_pct`, `provider_id`, `start`, `end`, `created_at`, `updated_at`) VALUES ( ?, ?, ?, ?, ?, ? )")

				if err != nil {
					fmt.Printf("could not execute query..")
					fmt.Println("error: " + err.Error())
					continue
				}
				providerId, err :=strconv.Atoi( provider )
				if err != nil {
					fmt.Println("error: " + err.Error())
					continue
				}
				defer stmt.Close()
				_, err = stmt.Exec(
					strconv.Itoa( metric.AvgAnswerRate ),
					strconv.Itoa( metric.AvgCallDuration ),
					strconv.Itoa( metric.FailureResponsePct ),
					providerId,
					start,
					deadline,
					now,
					now)
				if err != nil {
					fmt.Printf("could not execute query..")
					fmt.Println("error: " + err.Error())
					continue
				}

			}
			// reset metrics
			metrics=make( map[string]*MetricsData)
		}
	}


}