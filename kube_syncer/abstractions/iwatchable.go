package abstractions

type IWatchable interface {
	Watch() <-chan ISynchronizable
}
