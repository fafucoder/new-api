package maas

var ModelList = []string{
	"doubao-seedance-2.0",
	"doubao-seedance-1.0-pro",
	"doubao-seedance-1.0-lite",
}

const ChannelName = "maas-seedance"

const (
	MappingQueryEndpoint = "/mapping/query"
	CreateTaskEndpoint   = "/contents/generations/tasks"
	QueryTaskEndpoint    = "/contents/generations/tasks/%s"
)
