package common

func GetTrustQuota() int {
	if PreConsumeTrustQuota < 0 {
		return 0
	}
	return PreConsumeTrustQuota
}
