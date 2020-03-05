package plugin

import (
	"fmt"
	"hash/fnv"
	"reflect"

	"github.com/bluemedora/bplogagent/bundle"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

var PluginConfigDefinitions = make(map[string]func() PluginConfig)

// RegisterConfig will register a config struct by name in the packages config registry
// during package load time.
func RegisterConfig(name string, config PluginConfig) {
	PluginConfigDefinitions[name] = func() PluginConfig {
		val := reflect.New(reflect.TypeOf(config).Elem()).Interface()
		return val.(PluginConfig)
	}
}

type PluginConfig interface {
	ID() PluginID
	Type() string
	Build(BuildContext) (Plugin, error)
}

type OutputterConfig interface {
	PluginConfig
	Outputs() []PluginID
}

type InputterConfig interface {
	PluginConfig
	IsInputter()
}

type BuildContext struct {
	Plugins      map[PluginID]Plugin
	Bundles      []*bundle.BundleDefinition
	BundleInput  EntryChannel
	BundleOutput EntryChannel
	Logger       *zap.SugaredLogger
}

type pluginConfigNode struct {
	config PluginConfig
}

func (n pluginConfigNode) OutputIDs() map[PluginID]int64 {
	outputterConfig, ok := n.config.(OutputterConfig)
	if !ok {
		return nil
	}

	ids := make(map[PluginID]int64, 0)
	for _, outputID := range outputterConfig.Outputs() {
		h := fnv.New64a()
		h.Write([]byte(outputID))
		ids[outputID] = int64(h.Sum64())
	}
	return ids
}

func (n pluginConfigNode) ID() int64 {
	h := fnv.New64a()
	h.Write([]byte(n.config.ID()))
	return int64(h.Sum64())
}

func (n pluginConfigNode) DOTID() string {
	return string(n.config.ID())
}

func BuildPlugins(configs []PluginConfig, buildContext BuildContext) ([]Plugin, error) {
	// Construct the graph from the configs
	configGraph := simple.NewDirectedGraph()
	err := addConfigsToGraph(configGraph, configs)
	if err != nil {
		return nil, fmt.Errorf("failed to build config graph: %s", err)
	}

	marshalled, err := dot.Marshal(configGraph, "G", "", " ")
	if err != nil {
		buildContext.Logger.Info("Failed to marshal the config graph: %s", err)
	}
	buildContext.Logger.Info("Created a graph:\n", string(marshalled))

	// Sort the configs topologically by outputs
	// This will fail if the graph is not acyclic
	sortedNodes, err := topo.Sort(configGraph)
	if err != nil {
		// TODO make this error message more user-readable
		return nil, fmt.Errorf("failed to order plugin dependencies: %s", err)
	}

	// Build the configs in reverse topological order
	// Plugins contains all the plugins built so far, so building
	// outputs first, and working backwards should mean all outputs
	// already exist by the time each plugin is built
	buildContext.Plugins = make(map[PluginID]Plugin)
	for i := len(sortedNodes) - 1; i >= 0; i-- { // iterate backwards
		node := sortedNodes[i]
		configNode, ok := node.(pluginConfigNode)
		if !ok {
			panic("a node was found in the graph that is not a pluginConfigNode")
		}

		plugin, err := configNode.config.Build(buildContext)
		if err != nil {
			return nil, fmt.Errorf("failed to build plugin with id '%s': %s", configNode.config.ID(), err)
		}

		buildContext.Plugins[plugin.ID()] = plugin
	}

	// Warn if there is an inputter that has no outputters sending to it
	for _, node := range sortedNodes {
		if _, ok := node.(pluginConfigNode).config.(InputterConfig); ok {
			outputters := configGraph.To(node.ID())
			if outputters.Len() == 0 {
				buildContext.Logger.Warnw("Inputter has no outputs sending to it", "plugin_id", node.(pluginConfigNode).config.ID())
			}
		}
	}

	// Convert from a map to a slice
	pluginSlice := make([]Plugin, 0, len(buildContext.Plugins))
	for _, plugin := range buildContext.Plugins {
		pluginSlice = append(pluginSlice, plugin)
	}

	return pluginSlice, nil
}

func addConfigsToGraph(configGraph *simple.DirectedGraph, configs []PluginConfig) error {
	// Build nodes
	configNodes := make([]pluginConfigNode, 0, len(configs))
	for _, config := range configs {
		configNodes = append(configNodes, pluginConfigNode{config})
	}

	// Add nodes to graph
	seenIDs := make(map[int64]struct{})
	for _, node := range configNodes {
		// Check that the node ID is unique
		if _, ok := seenIDs[node.ID()]; ok {
			return fmt.Errorf("multiple configs found with id '%s'", node.config.ID())
		} else {
			seenIDs[node.ID()] = struct{}{}
		}
		configGraph.AddNode(node)
	}

	// Connect graph
	for _, node := range configNodes {
		for outputID, outputNodeID := range node.OutputIDs() {
			outputNode := configGraph.Node(outputNodeID)
			if outputNode == nil {
				return fmt.Errorf("failed to find node for output ID %s", outputID)
			}
			edge := configGraph.NewEdge(node, outputNode)
			configGraph.SetEdge(edge)
		}
	}

	return nil
}

func UnmarshalHook(c *mapstructure.DecoderConfig) {
	c.DecodeHook = newPluginConfigDecoder()
}

func newPluginConfigDecoder() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		var m map[interface{}]interface{}
		if f != reflect.TypeOf(m) {
			return data, nil
		}

		if t.String() != "plugin.PluginConfig" {
			return data, nil
		}

		d, ok := data.(map[interface{}]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected data type %T for plugin config", data)
		}

		typeInterface, ok := d["type"]
		if !ok {
			return nil, fmt.Errorf("missing type field for plugin config")
		}

		typeString, ok := typeInterface.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T for plugin config type", typeInterface)
		}

		configDefinitionFunc, ok := PluginConfigDefinitions[typeString]
		if !ok {
			return nil, fmt.Errorf("unknown plugin config type %s", typeString)
		}

		configDefinition := configDefinitionFunc()
		// TODO handle unused keys
		err := mapstructure.Decode(data, &configDefinition)
		if err != nil {
			return nil, fmt.Errorf("failed to decode plugin definition: %s", err)
		}

		return configDefinition, nil
	}
}
