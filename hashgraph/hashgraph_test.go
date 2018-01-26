package hashgraph

import (
	"crypto/ecdsa"
	"fmt"
	"testing"
    "os"
	"strings"

	"reflect"

	"math"

	"github.com/dindinw/dagproject/hashgraph/crypto"
)

var (
	cacheSize = 100
	MAX_NODES = 3
)

type Node struct {
	ID     int
	Pub    []byte
	PubHex string
	Key    *ecdsa.PrivateKey
	Events []Event
}

func NewNode(key *ecdsa.PrivateKey, id int) Node {
	pub := crypto.FromECDSAPub(&key.PublicKey)
	node := Node{
		ID:     id,
		Key:    key,
		Pub:    pub,
		PubHex: fmt.Sprintf("0x%X", pub),
		Events: []Event{},
	}
	return node
}
func (node *Node) signAndAddEvent(event Event, name string, index map[string]string, orderedEvents *[]Event) {
	event.Sign(node.Key)
	node.Events = append(node.Events, event)
	index[name] = event.Hex()
	*orderedEvents = append(*orderedEvents, event)
}
func (node *Node) dump(){
	fmt.Fprintf(os.Stdout,"node : %+v \n", node)
}

type play struct {
	to          int
	index       int
	selfParent  string
	otherParent string
	name        string
	payload     [][]byte
}
func (play *play) dump(){
	fmt.Fprintf(os.Stdout,"play : %+v \n", play)
}

/*
|  e12  |
|   | \ |
|  s10   e20
|   | / |
|   /   |
| / |   |
s00 |  s20
|   |   |
e01 |   |
| \ |   |
e0  e1  e2
0   1   2
*/
func initHashgraph(t *testing.T) (Hashgraph, map[string]string) {
	index := make(map[string]string)  // node_name -> event.hex (hash)
	nodes := []Node{}
	orderedEvents := &[]Event{}       // 

    // init node 0, 1, 2 with e0, e1, e2
	for i := 0; i < MAX_NODES; i++ {
		key, _ := crypto.GenerateECDSAKey()
		node := NewNode(key, i)
		event := NewEvent([][]byte{}, []string{"", ""}, node.Pub, 0)
		node.signAndAddEvent(event, fmt.Sprintf("e%d", i), index, orderedEvents)
		nodes = append(nodes, node)
	}

	//for _, n := range nodes { n.dump() }

 	/*
	for i,v := range index {
		fmt.Fprintf(os.Stdout,"index : %v -> %v \n",i,v)
	}
 	*/

	
	//fmt.Fprintf(os.Stdout,"index : %+v\n",index)

	plays := []play{
		play{0, 1, "e0", "e1", "e01", [][]byte{}},
		play{2, 1, "e2", "", "s20", [][]byte{}},
		play{1, 1, "e1", "", "s10", [][]byte{}},
		play{0, 2, "e01", "", "s00", [][]byte{}},
		play{2, 2, "s20", "s00", "e20", [][]byte{}},
		play{1, 2, "s10", "e20", "e12", [][]byte{}},
	}
	/*
	for _, p := range plays {
		p.dump()
	}
	*/
	// init events by using play data
	for _, p := range plays {

		fmt.Fprintf(os.Stdout,"init event from play : [ selfParent %v -> %v, otherParent %v -> %v ]\n",
			p.selfParent, index[p.selfParent],
			p.otherParent, index[p.otherParent])
		fmt.Fprintf(os.Stdout,"init event from play : nodes[to:%v] -> ID=%v,Event_Count=%v,Event=%+v\n",p.to, nodes[p.to].ID,
			len(nodes[p.to].Events),nodes[p.to].Events)
		for i,v := range index {
			fmt.Fprintf(os.Stdout,"init event from play :   index %v -> %v \n",i,v)
		}


		// create event by using play data
		e := NewEvent(p.payload,
			[]string{index[p.selfParent], index[p.otherParent]},
			nodes[p.to].Pub,
			p.index)
		// sign & add event to index and save to orderedEvents
		nodes[p.to].signAndAddEvent(e, p.name, index, orderedEvents)
	}

	participants := make(map[string]int)
	for _, node := range nodes {
		participants[node.PubHex] = node.ID
	}

	store := NewInmemStore(participants, cacheSize)
	h := NewHashgraph(participants, store, nil)
	for i, ev := range *orderedEvents {
		if err := h.InitEventCoordinates(&ev); err != nil {
			t.Fatalf("%d: %s", i, err)
		}

		if err := h.Store.SetEvent(ev); err != nil {
			t.Fatalf("%d: %s", i, err)
		}

		if err := h.UpdateAncestorFirstDescendant(ev); err != nil {
			t.Fatalf("%d: %s", i, err)
		}

	}
    /*
	fmt.Fprintf(os.Stdout, "Hashgraph : %+v\n",h)
	fmt.Fprintf(os.Stdout, "Index     : %+v\n",index)
	fmt.Fprintf(os.Stdout, "OrderedEvents : %+v\n", orderedEvents)
    */

	return h, index
}

func TestInitEventCoordinates (t *testing.T){
	index := make(map[string]string)
	nodes := []Node{}
	orderedEvents := &[]Event{}

	for i := 0; i < MAX_NODES; i++ {
		key, _ := crypto.GenerateECDSAKey()
		node := NewNode(key, i)
		event := NewEvent([][]byte{}, []string{"", ""}, node.Pub, 0)
		node.signAndAddEvent(event, fmt.Sprintf("e%d", i), index, orderedEvents)
		nodes = append(nodes, node)
	}
	e := NewEvent([][]byte{},
		[]string{"", index["e0"]},
		nodes[1].Pub,
		1);

	for i,v := range index {
		fmt.Fprintf(os.Stdout,"TestInitEventCoordinates :   index %v -> %v \n",i,v)
	}
	for _,e := range *orderedEvents {
		fmt.Fprintf(os.Stdout,"TestInitEventCoordinates : event %+v \n",e)
	}
	fmt.Fprintf(os.Stdout,"TestInitEventCoordinates : event %+v \n",e)

	nodes[1].signAndAddEvent(e, "e01", index, orderedEvents)
	participants := make(map[string]int)
	for _, node := range nodes {
		participants[node.PubHex] = node.ID
	}
	hashgraph := NewHashgraph(participants, NewInmemStore(participants, cacheSize), nil)
	hashgraph.InitEventCoordinates(&e)
}

func TestAncestor(t *testing.T) {
	h, index := initHashgraph(t)

	//1 generation
	if !h.Ancestor(index["e01"], index["e0"]) {
		t.Fatal("e0 should be ancestor of e01")
	}
	if !h.Ancestor(index["e01"], index["e1"]) {
		t.Fatal("e1 should be ancestor of e01")
	}
	if !h.Ancestor(index["s00"], index["e01"]) {
		t.Fatal("e01 should be ancestor of s00")
	}
	if !h.Ancestor(index["s20"], index["e2"]) {
		t.Fatal("e2 should be ancestor of s20")
	}
	if !h.Ancestor(index["e20"], index["s00"]) {
		t.Fatal("s00 should be ancestor of e20")
	}
	if !h.Ancestor(index["e20"], index["s20"]) {
		t.Fatal("s20 should be ancestor of e20")
	}
	if !h.Ancestor(index["e12"], index["e20"]) {
		t.Fatal("e20 should be ancestor of e12")
	}
	if !h.Ancestor(index["e12"], index["s10"]) {
		t.Fatal("s10 should be ancestor of e12")
	}

	//2 generations
	if !h.Ancestor(index["s00"], index["e0"]) {
		t.Fatalf("e0 should be ancestor of s00")
	}
	if !h.Ancestor(index["s00"], index["e1"]) {
		t.Fatalf("e1 should be ancestor of s00")
	}
	if !h.Ancestor(index["e20"], index["e01"]) {
		t.Fatalf("e01 should be ancestor of e20")
	}
	if !h.Ancestor(index["e20"], index["e2"]) {
		t.Fatalf("e2 should be ancestor of e20")
	}
	if !h.Ancestor(index["e12"], index["e1"]) {
		t.Fatalf("e1 should be ancestor of e12")
	}
	if !h.Ancestor(index["e12"], index["s20"]) {
		t.Fatalf("s20 should be ancestor of e12")
	}

	//3 generations
	if !h.Ancestor(index["e20"], index["e0"]) {
		t.Fatal("e0 should be ancestor of e20")
	}
	if !h.Ancestor(index["e20"], index["e1"]) {
		t.Fatal("e1 should be ancestor of e20")
	}
	if !h.Ancestor(index["e20"], index["e2"]) {
		t.Fatal("e2 should be ancestor of e20")
	}
	if !h.Ancestor(index["e12"], index["e01"]) {
		t.Fatal("e01 should be ancestor of e12")
	}
	if !h.Ancestor(index["e12"], index["e0"]) {
		t.Fatal("e0 should be ancestor of e12")
	}
	if !h.Ancestor(index["e12"], index["e1"]) {
		t.Fatal("e1 should be ancestor of e12")
	}
	if !h.Ancestor(index["e12"], index["e2"]) {
		t.Fatal("e2 should be ancestor of e12")
	}

	//false positive
	if h.Ancestor(index["e01"], index["e2"]) {
		t.Fatal("e2 should not be ancestor of e01")
	}
	if h.Ancestor(index["s00"], index["e2"]) {
		t.Fatal("e2 should not be ancestor of s00")
	}

	if h.Ancestor(index["e0"], "") {
		t.Fatal("\"\" should not be ancestor of e0")
	}
	if h.Ancestor(index["s00"], "") {
		t.Fatal("\"\" should not be ancestor of s00")
	}
	if h.Ancestor(index["e12"], "") {
		t.Fatal("\"\" should not be ancestor of e12")
	}

}

func TestSelfAncestor(t *testing.T) {
	h, index := initHashgraph(t)

	//1 generation
	if !h.SelfAncestor(index["e01"], index["e0"]) {
		t.Fatal("e0 should be self ancestor of e01")
	}
	if !h.SelfAncestor(index["s00"], index["e01"]) {
		t.Fatal("e01 should be self ancestor of s00")
	}

	//1 generation false negatives
	if h.SelfAncestor(index["e01"], index["e1"]) {
		t.Fatal("e1 should not be self ancestor of e01")
	}
	if h.SelfAncestor(index["e12"], index["e20"]) {
		t.Fatal("e20 should not be self ancestor of e12")
	}
	if h.SelfAncestor(index["s20"], "") {
		t.Fatal("\"\" should not be self ancestor of s20")
	}

	//2 generation
	if !h.SelfAncestor(index["e20"], index["e2"]) {
		t.Fatal("e2 should be self ancestor of e20")
	}
	if !h.SelfAncestor(index["e12"], index["e1"]) {
		t.Fatal("e1 should be self ancestor of e12")
	}

	//2 generation false negative
	if h.SelfAncestor(index["e20"], index["e0"]) {
		t.Fatal("e0 should not be self ancestor of e20")
	}
	if h.SelfAncestor(index["e12"], index["e2"]) {
		t.Fatal("e2 should not be self ancestor of e12")
	}
	if h.SelfAncestor(index["e20"], index["e01"]) {
		t.Fatal("e01 should not be self ancestor of e20")
	}

}

func TestSee(t *testing.T) {
	h, index := initHashgraph(t)

	if !h.See(index["e01"], index["e0"]) {
		t.Fatal("e01 should see e0")
	}
	if !h.See(index["e01"], index["e1"]) {
		t.Fatal("e01 should see e1")
	}
	if !h.See(index["e20"], index["e0"]) {
		t.Fatal("e20 should see e0")
	}
	if !h.See(index["e20"], index["e01"]) {
		t.Fatal("e20 should see e01")
	}
	if !h.See(index["e12"], index["e01"]) {
		t.Fatal("e12 should see e01")
	}
	if !h.See(index["e12"], index["e0"]) {
		t.Fatal("e12 should see e0")
	}
	if !h.See(index["e12"], index["e1"]) {
		t.Fatal("e12 should see e1")
	}
	if !h.See(index["e12"], index["s20"]) {
		t.Fatal("e12 should see s20")
	}
}

/*
|    |    e20
|    |   / |
|    | /   |
|    /     |
|  / |     |
e01  |     |
| \  |     |
|   \|     |
|    |\    |
|    |  \  |
e0   e1 (a)e2
0    1     2

Node 2 Forks; events a and e2 are both created by node2, they are not self-parents
and yet they are both ancestors of event e20
*/
func TestFork(t *testing.T) {
	index := make(map[string]string)
	nodes := []Node{}

	participants := make(map[string]int)
	for _, node := range nodes {
		participants[node.PubHex] = node.ID
	}

	store := NewInmemStore(participants, cacheSize)
	hashgraph := NewHashgraph(participants, store, nil)

	for i := 0; i < MAX_NODES; i++ {
		key, _ := crypto.GenerateECDSAKey()
		node := NewNode(key, i)
		event := NewEvent([][]byte{}, []string{"", ""}, node.Pub, 0)
		event.Sign(node.Key)
		index[fmt.Sprintf("e%d", i)] = event.Hex()
		hashgraph.InsertEvent(event)
		nodes = append(nodes, node)
	}

	//a and e2 need to have different hashes
	eventA := NewEvent([][]byte{[]byte("yo")}, []string{"", ""}, nodes[2].Pub, 0)
	eventA.Sign(nodes[2].Key)
	index["a"] = eventA.Hex()
	if err := hashgraph.InsertEvent(eventA); err == nil {
		t.Fatal("InsertEvent should return error for 'a'")
	}

	event01 := NewEvent([][]byte{},
		[]string{index["e0"], index["a"]}, //e0 and a
		nodes[0].Pub, 1)
	event01.Sign(nodes[0].Key)
	index["e01"] = event01.Hex()
	if err := hashgraph.InsertEvent(event01); err == nil {
		t.Fatal("InsertEvent should return error for e01")
	}

	event20 := NewEvent([][]byte{},
		[]string{index["e2"], index["e01"]}, //e2 and e01
		nodes[2].Pub, 1)
	event20.Sign(nodes[2].Key)
	index["e20"] = event20.Hex()
	if err := hashgraph.InsertEvent(event20); err == nil {
		t.Fatal("InsertEvent should return error for e20")
	}
}

/*
|  s11  |
|   |   |
|   f1  |
|  /|   |
| / s10 |
|/  |   |
e02 |   |
| \ |   |
|   \   |
|   | \ |
s00 |  e21
|   | / |
|  e10  s20
| / |   |
e0  e1  e2
0   1    2
*/
func initRoundHashgraph(t *testing.T) (Hashgraph, map[string]string) {
	index := make(map[string]string)
	nodes := []Node{}
	orderedEvents := &[]Event{}

	for i := 0; i < MAX_NODES; i++ {
		key, _ := crypto.GenerateECDSAKey()
		node := NewNode(key, i)
		event := NewEvent([][]byte{}, []string{"", ""}, node.Pub, 0)
		node.signAndAddEvent(event, fmt.Sprintf("e%d", i), index, orderedEvents)
		nodes = append(nodes, node)
	}
	// to -> node id
	// index -> the seq of the event created by the node
	
	plays := []play{
		play{1, 1, "e1", "e0", "e10", [][]byte{}},
		play{2, 1, "e2", "", "s20", [][]byte{}},
		play{0, 1, "e0", "", "s00", [][]byte{}},
		play{2, 2, "s20", "e10", "e21", [][]byte{}},
		play{0, 2, "s00", "e21", "e02", [][]byte{}},
		play{1, 2, "e10", "", "s10", [][]byte{}},
		play{1, 3, "s10", "e02", "f1", [][]byte{}},
		play{1, 4, "f1", "", "s11", [][]byte{[]byte("abc")}},
	}

	for _, p := range plays {
		e := NewEvent(p.payload,
			[]string{index[p.selfParent], index[p.otherParent]},
			nodes[p.to].Pub,
			p.index)
		nodes[p.to].signAndAddEvent(e, p.name, index, orderedEvents)
	}

	participants := make(map[string]int)
	for _, node := range nodes {
		participants[node.PubHex] = node.ID
	}

	hashgraph := NewHashgraph(participants, NewInmemStore(participants, cacheSize), nil)
	for i, ev := range *orderedEvents {
		if err := hashgraph.InsertEvent(ev); err != nil {
			fmt.Printf("ERROR inserting event %d: %s\n", i, err)
		}
	}
	return hashgraph, index
}

func TestInsertEvent(t *testing.T) {
	h, index := initRoundHashgraph(t)

	expectedFirstDescendants := make([]EventCoordinates, MAX_NODES)
	expectedLastAncestors := make([]EventCoordinates, MAX_NODES)

	//e0
	e0, err := h.Store.GetEvent(index["e0"])
	if err != nil {
		t.Fatal(err)
	}

	if !(e0.Body.selfParentIndex == -1 &&
		e0.Body.otherParentCreatorID == -1 &&
		e0.Body.otherParentIndex == -1 &&
		e0.Body.creatorID == h.Participants[e0.Creator()]) {
		t.Fatalf("Invalid wire info on e0")
	}

	expectedFirstDescendants[0] = EventCoordinates{
		index: 0,
		hash:  index["e0"],
	}
	expectedFirstDescendants[1] = EventCoordinates{
		index: 1,
		hash:  index["e10"],
	}
	expectedFirstDescendants[2] = EventCoordinates{
		index: 2,
		hash:  index["e21"],
	}

	expectedLastAncestors[0] = EventCoordinates{
		index: 0,
		hash:  index["e0"],
	}
	expectedLastAncestors[1] = EventCoordinates{
		index: -1,
	}
	expectedLastAncestors[2] = EventCoordinates{
		index: -1,
	}

	if !reflect.DeepEqual(e0.firstDescendants, expectedFirstDescendants) {
		t.Fatal("e0 firstDescendants not good")
	}
	if !reflect.DeepEqual(e0.lastAncestors, expectedLastAncestors) {
		t.Fatal("e0 lastAncestors not good")
	}

	//e21
	e21, err := h.Store.GetEvent(index["e21"])
	if err != nil {
		t.Fatal(err)
	}

	e10, err := h.Store.GetEvent(index["e10"])
	if err != nil {
		t.Fatal(err)
	}

	if !(e21.Body.selfParentIndex == 1 &&
		e21.Body.otherParentCreatorID == h.Participants[e10.Creator()] &&
		e21.Body.otherParentIndex == 1 &&
		e21.Body.creatorID == h.Participants[e21.Creator()]) {
		t.Fatalf("Invalid wire info on e21")
	}

	expectedFirstDescendants[0] = EventCoordinates{
		index: 2,
		hash:  index["e02"],
	}
	expectedFirstDescendants[1] = EventCoordinates{
		index: 3,
		hash:  index["f1"],
	}
	expectedFirstDescendants[2] = EventCoordinates{
		index: 2,
		hash:  index["e21"],
	}

	expectedLastAncestors[0] = EventCoordinates{
		index: 0,
		hash:  index["e0"],
	}
	expectedLastAncestors[1] = EventCoordinates{
		index: 1,
		hash:  index["e10"],
	}
	expectedLastAncestors[2] = EventCoordinates{
		index: 2,
		hash:  index["e21"],
	}

	if !reflect.DeepEqual(e21.firstDescendants, expectedFirstDescendants) {
		t.Fatal("e21 firstDescendants not good")
	}
	if !reflect.DeepEqual(e21.lastAncestors, expectedLastAncestors) {
		t.Fatal("e21 lastAncestors not good")
	}

	//f1
	f1, err := h.Store.GetEvent(index["f1"])
	if err != nil {
		t.Fatal(err)
	}

	if !(f1.Body.selfParentIndex == 2 &&
		f1.Body.otherParentCreatorID == h.Participants[e0.Creator()] &&
		f1.Body.otherParentIndex == 2 &&
		f1.Body.creatorID == h.Participants[f1.Creator()]) {
		t.Fatalf("Invalid wire info on f1")
	}

	expectedFirstDescendants[0] = EventCoordinates{
		index: math.MaxInt64,
	}
	expectedFirstDescendants[1] = EventCoordinates{
		index: 3,
		hash:  index["f1"],
	}
	expectedFirstDescendants[2] = EventCoordinates{
		index: math.MaxInt64,
	}

	expectedLastAncestors[0] = EventCoordinates{
		index: 2,
		hash:  index["e02"],
	}
	expectedLastAncestors[1] = EventCoordinates{
		index: 3,
		hash:  index["f1"],
	}
	expectedLastAncestors[2] = EventCoordinates{
		index: 2,
		hash:  index["e21"],
	}

	if !reflect.DeepEqual(f1.firstDescendants, expectedFirstDescendants) {
		t.Fatal("f1 firstDescendants not good")
	}
	if !reflect.DeepEqual(f1.lastAncestors, expectedLastAncestors) {
		t.Fatal("f1 lastAncestors not good")
	}

	//Pending loaded Events
	if ple := h.PendingLoadedEvents; ple != 4 {
		t.Fatalf("PendingLoadedEvents should be 4, not %d", ple)
	}

}

func TestReadWireInfo(t *testing.T) {
	h, index := initRoundHashgraph(t)

	for k, evh := range index {
		ev, err := h.Store.GetEvent(evh)
		if err != nil {
			t.Fatal(err)
		}

		evWire := ev.ToWire()

		evFromWire, err := h.ReadWireInfo(evWire)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(ev.Body, evFromWire.Body) {
			t.Fatalf("Error converting %s.Body from light wire", k)
		}

		if !reflect.DeepEqual(ev.R, evFromWire.R) {
			t.Fatalf("Error converting %s.R from light wire", k)
		}

		if !reflect.DeepEqual(ev.S, evFromWire.S) {
			t.Fatalf("Error converting %s.S from light wire", k)
		}

		ok, err := ev.Verify()
		if !ok {
			t.Fatalf("Error verifying signature for %s from ligh wire: %v", k, err)
		}
	}
}

func TestStronglySee(t *testing.T) {
	h, index := initRoundHashgraph(t)

	if !h.StronglySee(index["e21"], index["e0"]) {
		t.Fatal("e21 should strongly see e0")
	}

	if !h.StronglySee(index["e02"], index["e10"]) {
		t.Fatal("e02 should strongly see e10")
	}
	if !h.StronglySee(index["e02"], index["e0"]) {
		t.Fatal("e02 should strongly see e0")
	}
	if !h.StronglySee(index["e02"], index["e1"]) {
		t.Fatal("e02 should strongly see e1")
	}

	if !h.StronglySee(index["f1"], index["e21"]) {
		t.Fatal("f1 should strongly see e21")
	}
	if !h.StronglySee(index["f1"], index["e10"]) {
		t.Fatal("f1 should strongly see e10")
	}
	if !h.StronglySee(index["f1"], index["e0"]) {
		t.Fatal("f1 should strongly see e0")
	}
	if !h.StronglySee(index["f1"], index["e1"]) {
		t.Fatal("f1 should strongly see e1")
	}
	if !h.StronglySee(index["f1"], index["e2"]) {
		t.Fatal("f1 should strongly see e2")
	}
	if !h.StronglySee(index["s11"], index["e2"]) {
		t.Fatal("s11 should strongly see e2")
	}

	//false negatives
	if h.StronglySee(index["e10"], index["e0"]) {
		t.Fatal("e12 should not strongly see e2")
	}
	if h.StronglySee(index["e21"], index["e1"]) {
		t.Fatal("e21 should not strongly see e1")
	}
	if h.StronglySee(index["e21"], index["e2"]) {
		t.Fatal("e21 should not strongly see e2")
	}
	if h.StronglySee(index["e02"], index["e2"]) {
		t.Fatal("e02 should not strongly see e2")
	}
	if h.StronglySee(index["s11"], index["e02"]) {
		t.Fatal("s11 should not strongly see e02")
	}
}

func TestParentRound(t *testing.T) {
	h, index := initRoundHashgraph(t)

	round0Witnesses := make(map[string]RoundEvent)
	round0Witnesses[index["e0"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e1"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e2"]] = RoundEvent{Witness: true, Famous: Undefined}
	h.Store.SetRound(0, RoundInfo{Events: round0Witnesses})

	round1Witnesses := make(map[string]RoundEvent)
	round1Witnesses[index["f1"]] = RoundEvent{Witness: true, Famous: Undefined}
	h.Store.SetRound(1, RoundInfo{Events: round1Witnesses})

	if r := h.ParentRound(index["e0"]); r != 0 {
		t.Fatalf("parent round of e0 should be 0, not %d", r)
	}
	if r := h.ParentRound(index["e1"]); r != 0 {
		t.Fatalf("parent round of e1 should be 0, not %d", r)
	}
	if r := h.ParentRound(index["e10"]); r != 0 {
		t.Fatalf("parent round of e10 should be 0, not %d", r)
	}
	if r := h.ParentRound(index["f1"]); r != 0 {
		t.Fatalf("parent round of f1 should be 0, not %d", r)
	}
	if r := h.ParentRound(index["s11"]); r != 1 {
		t.Fatalf("parent round of s11 should be 1, not %d", r)
	}
}

func TestWitness(t *testing.T) {
	h, index := initRoundHashgraph(t)

	round0Witnesses := make(map[string]RoundEvent)
	round0Witnesses[index["e0"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e1"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e2"]] = RoundEvent{Witness: true, Famous: Undefined}
	h.Store.SetRound(0, RoundInfo{Events: round0Witnesses})

	round1Witnesses := make(map[string]RoundEvent)
	round1Witnesses[index["f1"]] = RoundEvent{Witness: true, Famous: Undefined}
	h.Store.SetRound(1, RoundInfo{Events: round1Witnesses})

	if !h.Witness(index["e0"]) {
		t.Fatalf("e0 should be witness")
	}
	if !h.Witness(index["e1"]) {
		t.Fatalf("e1 should be witness")
	}
	if !h.Witness(index["e2"]) {
		t.Fatalf("e2 should be witness")
	}
	if !h.Witness(index["f1"]) {
		t.Fatalf("f1 should be witness")
	}

	if h.Witness(index["e10"]) {
		t.Fatalf("e10 should not be witness")
	}
	if h.Witness(index["e21"]) {
		t.Fatalf("e21 should not be witness")
	}
	if h.Witness(index["e02"]) {
		t.Fatalf("e02 should not be witness")
	}
}

func TestRoundInc(t *testing.T) {
	h, index := initRoundHashgraph(t)

	round0Witnesses := make(map[string]RoundEvent)
	round0Witnesses[index["e0"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e1"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e2"]] = RoundEvent{Witness: true, Famous: Undefined}
	h.Store.SetRound(0, RoundInfo{Events: round0Witnesses})

	if !h.RoundInc(index["f1"]) {
		t.Fatal("RoundInc f1 should be true")
	}

	if h.RoundInc(index["e02"]) {
		t.Fatal("RoundInc e02 should be false because it doesnt strongly see e2")
	}
}

func TestRound(t *testing.T) {
	h, index := initRoundHashgraph(t)

	round0Witnesses := make(map[string]RoundEvent)
	round0Witnesses[index["e0"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e1"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e2"]] = RoundEvent{Witness: true, Famous: Undefined}
	h.Store.SetRound(0, RoundInfo{Events: round0Witnesses})

	if r := h.Round(index["f1"]); r != 1 {
		t.Fatalf("round of f1 should be 1 not %d", r)
	}
	if r := h.Round(index["e02"]); r != 0 {
		t.Fatalf("round of e02 should be 0 not %d", r)
	}

}

func TestRoundDiff(t *testing.T) {
	h, index := initRoundHashgraph(t)

	round0Witnesses := make(map[string]RoundEvent)
	round0Witnesses[index["e0"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e1"]] = RoundEvent{Witness: true, Famous: Undefined}
	round0Witnesses[index["e2"]] = RoundEvent{Witness: true, Famous: Undefined}
	h.Store.SetRound(0, RoundInfo{Events: round0Witnesses})

	if d, err := h.RoundDiff(index["f1"], index["e02"]); d != 1 {
		if err != nil {
			t.Fatalf("RoundDiff(f1, e02) returned an error: %s", err)
		}
		t.Fatalf("RoundDiff(f1, e02) should be 1 not %d", d)
	}

	if d, err := h.RoundDiff(index["e02"], index["f1"]); d != -1 {
		if err != nil {
			t.Fatalf("RoundDiff(e02, f1) returned an error: %s", err)
		}
		t.Fatalf("RoundDiff(e02, f1) should be -1 not %d", d)
	}
	if d, err := h.RoundDiff(index["e02"], index["e21"]); d != 0 {
		if err != nil {
			t.Fatalf("RoundDiff(e20, e21) returned an error: %s", err)
		}
		t.Fatalf("RoundDiff(e20, e21) should be 0 not %d", d)
	}
}

func TestDivideRounds(t *testing.T) {
	h, index := initRoundHashgraph(t)

	err := h.DivideRounds()
	if err != nil {
		t.Fatal(err)
	}

	if l := h.Store.Rounds(); l != 2 {
		t.Fatalf("length of rounds should be 2 not %d", l)
	}

	round0, err := h.Store.GetRound(0)
	if err != nil {
		t.Fatal(err)
	}
	if l := len(round0.Witnesses()); l != 3 {
		t.Fatalf("round 0 should have 3 witnesses, not %d", l)
	}
	if !contains(round0.Witnesses(), index["e0"]) {
		t.Fatalf("round 0 witnesses should contain e0")
	}
	if !contains(round0.Witnesses(), index["e1"]) {
		t.Fatalf("round 0 witnesses should contain e1")
	}
	if !contains(round0.Witnesses(), index["e2"]) {
		t.Fatalf("round 0 witnesses should contain e2")
	}

	round1, err := h.Store.GetRound(1)
	if err != nil {
		t.Fatal(err)
	}
	if l := len(round1.Witnesses()); l != 1 {
		t.Fatalf("round 1 should have 1 witness, not %d", l)
	}
	if !contains(round1.Witnesses(), index["f1"]) {
		t.Fatalf("round 1 witnesses should contain f1")
	}

}

func contains(s []string, x string) bool {
	for _, e := range s {
		if e == x {
			return true
		}
	}
	return false
}

/*
		h0  |   h2
		| \ | / |
		|   h1  |
		|  /|   |
		g02 |   |
		| \ |   |
		|   \   |
		|   | \ |
	---	o02 |  g21 //e02's other-parent is f21. This situation can happen with concurrency
	|	|   | / |
	|	|  g10  |
	|	| / |   |
	|	g0  |   g2
	|	| \ | / |
	|	|   g1  |
	|	|  /|   |
	|	f02b|   |
	|	|   |   |
	|	f02 |   |
	|	| \ |   |
	|	|   \   |
	|	|   | \ |
	----------- f21
		|   | / |
		|  f10  |
		| / |   |
		f0  |   f2
		| \ | / |
		|  f1b  |
		|   |   |
		|   f1  |
		|  /|   |
		e02 |   |
		| \ |   |
		|   \   |
		|   | \ |
		|   |  e21b
		|   |   |
		|   |  e21
		|   | / |
		|  e10  |
		| / |   |
		e0  e1  e2
		0   1    2
*/
func initConsensusHashgraph() (Hashgraph, map[string]string) {
	index := make(map[string]string)
	nodes := []Node{}
	orderedEvents := &[]Event{}

	for i := 0; i < MAX_NODES; i++ {
		key, _ := crypto.GenerateECDSAKey()
		node := NewNode(key, i)
		event := NewEvent([][]byte{}, []string{"", ""}, node.Pub, 0)
		node.signAndAddEvent(event, fmt.Sprintf("e%d", i), index, orderedEvents)
		nodes = append(nodes, node)
	}

	plays := []play{
		play{1, 1, "e1", "e0", "e10", [][]byte{}},
		play{2, 1, "e2", "e10", "e21", [][]byte{[]byte("e21")}},
		play{2, 2, "e21", "", "e21b", [][]byte{}},
		play{0, 1, "e0", "e21b", "e02", [][]byte{}},
		play{1, 2, "e10", "e02", "f1", [][]byte{}},
		play{1, 3, "f1", "", "f1b", [][]byte{[]byte("f1b")}},
		play{0, 2, "e02", "f1b", "f0", [][]byte{}},
		play{2, 3, "e21b", "f1b", "f2", [][]byte{}},
		play{1, 4, "f1b", "f0", "f10", [][]byte{}},
		play{2, 4, "f2", "f10", "f21", [][]byte{}},
		play{0, 3, "f0", "f21", "f02", [][]byte{}},
		play{0, 4, "f02", "", "f02b", [][]byte{[]byte("e21")}},
		play{1, 5, "f10", "f02b", "g1", [][]byte{}},
		play{0, 5, "f02b", "g1", "g0", [][]byte{}},
		play{2, 5, "f21", "g1", "g2", [][]byte{}},
		play{1, 6, "g1", "g0", "g10", [][]byte{}},
		play{0, 6, "g0", "f21", "o02", [][]byte{}},
		play{2, 6, "g2", "g10", "g21", [][]byte{}},
		play{0, 7, "o02", "g21", "g02", [][]byte{}},
		play{1, 7, "g10", "g02", "h1", [][]byte{}},
		play{0, 8, "g02", "h1", "h0", [][]byte{}},
		play{2, 7, "g21", "h1", "h2", [][]byte{}},
	}

	for _, p := range plays {
		e := NewEvent(p.payload,
			[]string{index[p.selfParent], index[p.otherParent]},
			nodes[p.to].Pub,
			p.index)
		nodes[p.to].signAndAddEvent(e, p.name, index, orderedEvents)
	}

	participants := make(map[string]int)
	for _, node := range nodes {
		participants[node.PubHex] = node.ID
	}

	hashgraph := NewHashgraph(participants, NewInmemStore(participants, cacheSize), nil)
	for i, ev := range *orderedEvents {
		if err := hashgraph.InsertEvent(ev); err != nil {
			fmt.Printf("ERROR inserting event %d: %s\n", i, err)
		}
	}
	return hashgraph, index
}

func TestDecideFame(t *testing.T) {
	h, index := initConsensusHashgraph()

	h.DivideRounds()
	h.DecideFame()

	if r := h.Round(index["g0"]); r != 2 {
		t.Fatalf("g0 round should be 2, not %d", r)
	}
	if r := h.Round(index["g1"]); r != 2 {
		t.Fatalf("g1 round should be 2, not %d", r)
	}
	if r := h.Round(index["g2"]); r != 2 {
		t.Fatalf("g2 round should be 2, not %d", r)
	}

	round0, err := h.Store.GetRound(0)
	if err != nil {
		t.Fatal(err)
	}
	if f := round0.Events[index["e0"]]; !(f.Witness && f.Famous == True) {
		t.Fatalf("e0 should be famous; got %v", f)
	}
	if f := round0.Events[index["e1"]]; !(f.Witness && f.Famous == True) {
		t.Fatalf("e1 should be famous; got %v", f)
	}
	if f := round0.Events[index["e2"]]; !(f.Witness && f.Famous == True) {
		t.Fatalf("e2 should be famous; got %v", f)
	}
}

func TestOldestSelfAncestorToSee(t *testing.T) {
	h, index := initConsensusHashgraph()

	if a := h.OldestSelfAncestorToSee(index["f0"], index["e1"]); a != index["e02"] {
		t.Fatalf("oldest self ancestor of f0 to see e1 should be e02 not %s", getName(index, a))
	}
	if a := h.OldestSelfAncestorToSee(index["f1"], index["e0"]); a != index["e10"] {
		t.Fatalf("oldest self ancestor of f1 to see e0 should be e10 not %s", getName(index, a))
	}
	if a := h.OldestSelfAncestorToSee(index["f1b"], index["e0"]); a != index["e10"] {
		t.Fatalf("oldest self ancestor of f1b to see e0 should be e10 not %s", getName(index, a))
	}
	if a := h.OldestSelfAncestorToSee(index["g2"], index["f1"]); a != index["f2"] {
		t.Fatalf("oldest self ancestor of g2 to see f1 should be f2 not %s", getName(index, a))
	}
	if a := h.OldestSelfAncestorToSee(index["e21"], index["e1"]); a != index["e21"] {
		t.Fatalf("oldest self ancestor of e20 to see e1 should be e21 not %s", getName(index, a))
	}
	if a := h.OldestSelfAncestorToSee(index["e2"], index["e1"]); a != "" {
		t.Fatalf("oldest self ancestor of e2 to see e1 should be '' not %s", getName(index, a))
	}
}

func TestDecideRoundReceived(t *testing.T) {
	h, index := initConsensusHashgraph()

	h.DivideRounds()
	h.DecideFame()
	h.DecideRoundReceived()

	for name, hash := range index {
		e, _ := h.Store.GetEvent(hash)
		if rune(name[0]) == rune('e') {
			if r := *e.roundReceived; r != 1 {
				t.Fatalf("%s round received should be 1 not %d", name, r)
			}
		}
	}

}

func TestFindOrder(t *testing.T) {
	h, index := initConsensusHashgraph()

	h.DivideRounds()
	h.DecideFame()
	h.FindOrder()

	for i, e := range h.ConsensusEvents() {
		t.Logf("consensus[%d]: %s\n", i, getName(index, e))
	}

	if l := len(h.ConsensusEvents()); l != 7 {
		t.Fatalf("length of consensus should be 7 not %d", l)
	}

	if ple := h.PendingLoadedEvents; ple != 2 {
		t.Fatalf("PendingLoadedEvents should be 2, not %d", ple)
	}

	consensusEvents := h.ConsensusEvents()

	if n := getName(index, consensusEvents[0]); n != "e0" {
		t.Fatalf("consensus[0] should be e0, not %s", n)
	}

	//events which have the same consensus timestamp are ordered by whitened signature
	//which is not deterministic.
	if n := getName(index, consensusEvents[6]); n != "e02" {
		t.Fatalf("consensus[6] should be e02, not %s", n)
	}

}

func BenchmarkFindOrder(b *testing.B) {
	for n := 0; n < b.N; n++ {
		//we do not want to benchmark the initialization code
		b.StopTimer()
		h, _ := initConsensusHashgraph()
		b.StartTimer()

		h.DivideRounds()
		h.DecideFame()
		h.FindOrder()
	}
}

func TestKnown(t *testing.T) {
	h, _ := initConsensusHashgraph()

	expectedKnown := map[int]int{
		0: 9,
		1: 8,
		2: 8,
	}

	known := h.Known()
	for _, id := range h.Participants {
		if l := known[id]; l != expectedKnown[id] {
			t.Fatalf("Known[%d] should be %d, not %d", id, expectedKnown[id], l)
		}
	}
}

/*







    |    |    |    |
	|    |    |    |w51 collects votes from w40, w41, w42 and w43.
    |   w51   |    |IT DECIDES YES
    |    |  \ |    |
	|    |   e23   |
    |    |    | \  |------------------------
    |    |    |   w43
    |    |    | /  | Round 4 is a Coin Round. No decision will be made.
    |    |   w42   |
    |    | /  |    | w40 collects votes from w33, w32 and w31. It votes yes.
    |   w41   |    | w41 collects votes from w33, w32 and w31. It votes yes.
	| /  |    |    | w42 collects votes from w30, w31, w32 and w33. It votes yes.
   w40   |    |    | w43 collects votes from w30, w31, w32 and w33. It votes yes.
    | \  |    |    |------------------------
    |   d13   |    | w30 collects votes from w20, w21, w22 and w23. It votes yes
    |    |  \ |    | w31 collects votes from w21, w22 and w23. It votes no
   w30   |    \    | w32 collects votes from w20, w21, w22 and w23. It votes yes
    | \  |    | \  | w33 collects votes from w20, w21, w22 and w23. It votes yes
    |   \     |   w33
    |    | \  |  / |Again, none of the witnesses in round 3 are able to decide.
    |    |   w32   |However, a strong majority votes yes
    |    |  / |    |
	|   w31   |    |
    |  / |    |    |--------------------------
   w20   |    |    | w23 collects votes from w11, w12 and w13. It votes no
    |  \ |    |    | w21 collects votes from w11, w12, and w13. It votes no
    |    \    |    | w22 collects votes from w11, w12, w13 and w14. It votes yes
    |    | \  |    | w20 collects votes from w11, w12, w13 and w14. It votes yes
    |    |   w22   |
    |    | /  |    | None of the witnesses in round 2 were able to decide.
    |   c10   |    | They voted according to the majority of votes they observed
    | /  |    |    | in round 1. The vote is split 2-2
   b00  w21   |    |
    |    |  \ |    |
    |    |    \    |
    |    |    | \  |
    |    |    |   w23
    |    |    | /  |------------------------
   w10   |   b21   |
	| \  | /  |    | w10 votes yes (it can see w00)
    |   w11   |    | w11 votes yes
    |    |  \ |    | w12 votes no  (it cannot see w00)
	|    |   w12   | w13 votes no
    |    |    | \  |
    |    |    |   w13
    |    |    | /  |------------------------
    |   a10  a21   | We want to decide the fame of w00
    |  / |  / |    |
    |/  a12   |    |
   a00   |  \ |    |
	|    |   a23   |
    |    |    | \  |
   w00  w01  w02  w03
	0	 1	  2	   3
*/

func initFunkyHashgraph() (Hashgraph, map[string]string) {
	index := make(map[string]string)
	nodes := []Node{}
	orderedEvents := &[]Event{}

	n := 4
	for i := 0; i < n; i++ {
		key, _ := crypto.GenerateECDSAKey()
		node := NewNode(key, i)
		event := NewEvent([][]byte{}, []string{"", ""}, node.Pub, 0)
		node.signAndAddEvent(event, fmt.Sprintf("w0%d", i), index, orderedEvents)
		nodes = append(nodes, node)
	}

	plays := []play{
		play{2, 1, "w02", "w03", "a23", [][]byte{}},
		play{1, 1, "w01", "a23", "a12", [][]byte{}},
		play{0, 1, "w00", "", "a00", [][]byte{}},
		play{1, 2, "a12", "a00", "a10", [][]byte{}},
		play{2, 2, "a23", "a12", "a21", [][]byte{}},
		play{3, 1, "w03", "a21", "w13", [][]byte{}},
		play{2, 3, "a21", "w13", "w12", [][]byte{}},
		play{1, 3, "a10", "w12", "w11", [][]byte{}},
		play{0, 2, "a00", "w11", "w10", [][]byte{}},
		play{2, 4, "w12", "w11", "b21", [][]byte{}},
		play{3, 2, "w13", "b21", "w23", [][]byte{}},
		play{1, 4, "w11", "w23", "w21", [][]byte{}},
		play{0, 3, "w10", "", "b00", [][]byte{}},
		play{1, 5, "w21", "b00", "c10", [][]byte{}},
		play{2, 5, "b21", "c10", "w22", [][]byte{}},
		play{0, 4, "b00", "w22", "w20", [][]byte{}},
		play{1, 6, "c10", "w20", "w31", [][]byte{}},
		play{2, 6, "w22", "w31", "w32", [][]byte{}},
		play{0, 5, "w20", "w32", "w30", [][]byte{}},
		play{3, 3, "w23", "w32", "w33", [][]byte{}},
		play{1, 7, "w31", "w33", "d13", [][]byte{}},
		play{0, 6, "w30", "d13", "w40", [][]byte{}},
		play{1, 8, "d13", "w40", "w41", [][]byte{}},
		play{2, 7, "w32", "w41", "w42", [][]byte{}},
		play{3, 4, "w33", "w42", "w43", [][]byte{}},
		play{2, 8, "w42", "w43", "e23", [][]byte{}},
		play{1, 9, "w41", "e23", "w51", [][]byte{}},
	}

	for _, p := range plays {
		e := NewEvent(p.payload,
			[]string{index[p.selfParent], index[p.otherParent]},
			nodes[p.to].Pub,
			p.index)
		nodes[p.to].signAndAddEvent(e, p.name, index, orderedEvents)
	}

	participants := make(map[string]int)
	for _, node := range nodes {
		participants[node.PubHex] = node.ID
	}

	hashgraph := NewHashgraph(participants, NewInmemStore(participants, cacheSize), nil)
	for i, ev := range *orderedEvents {
		if err := hashgraph.InsertEvent(ev); err != nil {
			fmt.Printf("ERROR inserting event %d: %s\n", i, err)
		}
	}
	return hashgraph, index
}

func TestFunkyHashgraphFame(t *testing.T) {
	h, index := initFunkyHashgraph()

	h.DivideRounds()

	if l := h.Store.Rounds(); l != 6 {
		t.Fatalf("length of rounds should be 6 not %d", l)
	}

	for r := 0; r < 6; r++ {
		round, err := h.Store.GetRound(r)
		if err != nil {
			t.Fatal(err)
		}
		witnessNames := []string{}
		for _, w := range round.Witnesses() {
			witnessNames = append(witnessNames, getName(index, w))
		}
		t.Logf("Round %d witnesses: %v", r, witnessNames)
	}

	h.DecideFame()

	//rounds 0,1 and two should be decided
	expectedUndecidedRounds := []int{4, 5}
	if !reflect.DeepEqual(expectedUndecidedRounds, h.UndecidedRounds) {
		t.Fatalf("UndecidedRounds should be %v, not %v", expectedUndecidedRounds, h.UndecidedRounds)
	}

}

func getName(index map[string]string, hash string) string {
	for name, h := range index {
		if h == hash {
			return name
		}
	}
	return ""
}

func disp(index map[string]string, events []string) string {
	names := []string{}
	for _, h := range events {
		names = append(names, getName(index, h))
	}
	return fmt.Sprintf("[%s]", strings.Join(names, " "))
}
