package utils

// var ContainerdClient *containerd.Client
// var ContainerdNamespaceContext context.Context

// func InitContainerdClient() error {
// 	client, err := containerd.New("/run/containerd/containerd.sock")
// 	if err != nil {
// 		logrus.Errorf("Init Containerd Client Error :%s", err.Error())
// 		return err
// 	}
// 	ContainerdClient = client
// 	ContainerdNamespaceContext = namespaces.WithNamespace(context.Background(), "emu")
// 	return nil
// }
