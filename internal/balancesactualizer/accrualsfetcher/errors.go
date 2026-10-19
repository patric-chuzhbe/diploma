package accrualsfetcher

const (
	fetchAccrualErr1 = "in internal/balancesactualizer/accrualsfetcher/accrualsfetcher.go/fetchAccrual(): error while `http.NewRequestWithContext()` calling: %w"

	fetchAccrualErr2 = "in internal/balancesactualizer/accrualsfetcher/accrualsfetcher.go/fetchAccrual(): error while `resp.Body.Close()` calling: %w"

	fetchAccrualErr3 = "in internal/balancesactualizer/accrualsfetcher/accrualsfetcher.go/fetchAccrual(): error while `f.client.Do()` calling: %w"

	fetchAccrualErr4 = "failed to parse response for order %s: %w"

	fetchAccrualErr5 = "unexpected status code %d for order %s"

	fetchAccrualErr6 = "the response order number (%s) is not equal to the target order number (%s)"
)
