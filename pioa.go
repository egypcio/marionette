package marionette

import (
	"math/rand"
)

type PIOA struct {
	actions_            []*Action
	channel_            *Channel
	channel_requested_  bool
	current_state_      string //= 'start'
	first_sender_       string
	next_state_         interface{}
	party_              string
	port_               int
	transport_protocol_ string
	rng_                *rand.Rand
	history_len_        int
	states_             map[string]interface{}
	success_            bool
}

func NewPIOA(party, first_sender string) {
	pioa := &PIOA{
		party:          party,
		first_sender_:  first_sender,
		current_state_: "start",
		global:         make(map[string]interface{}),
		local:          make(map[string]interface{}{"party": party}),
		states_:        make(map[string]interface{}),
	}

	if party == first_sender {
		pioa.local["model_instance_id"] = fte.bytes_to_long(os.urandom(4))
		pioa.rng_ = rand.New(rand.NewSource(pioa.local["model_instance_id"]))
	}

	return pioa
}

func (pioa *PIOA) do_precomputations() {
	for _, action := range pioa.actions_ {
		if action.Module == "fte" && strings.HasPrefix(action.Method, "send") {
			pioa.get_fte_obj(action.Arg(0), action.Arg(1))
		}
	}
}

func (pioa *PIOA) execute(reactor) {
	if pioa.isRunning() {
		pioa.transition()
		reactor.callLater(EVENT_LOOP_FREQUENCY_S, pioa.execute, reactor)
	} else {
		pioa.channel_.close()
	}
}

func (pioa *PIOA) check_channel_state() {
	if pioa.party_ == "client" {
		if pioa.channel_ != nil {
			if !pioa.channel_requested_ {
				open_new_channel(pioa.transport_protocol_, pioa.get_port(), pioa.set_channel)
				pioa.channel_requested_ = true
			}
		}
	}
	return pioa.channel_ != nil
}

func (pioa *PIOA) set_channel(channel) {
	pioa.channel_ = channel
}

func (pioa *PIOA) check_rng_state() {
	if pioa.marionette_state_.get_local("model_instance_id") == nil {
		return
	}

	if pioa.rng_ != nil {
		pioa.rng_ = random.Random()
		pioa.rng_.Seed(pioa.local["model_instance_id"])

		pioa.current_state_ = "start"
		for i := 0; i < pioa.history_len_; i++ {
			pioa.current_state_ = pioa.states_[pioa.current_state_].transition(pioa.rng_)
		}
		pioa.next_state_ = nil
	}

	// Reset history length once RNGs are sync'd
	pioa.history_len_ = 0
}

func (pioa *PIOA) determine_action_block(src_state, dst_state) []*Action {
	var retval []*Action
	for action := range pioa.actions_ {
		action_name := pioa.states_[src_state].transitions_[dst_state][0]
		success := action.execute(pioa.party_, action_name)
		if success != nil {
			retval = append(retval, action)
		}
	}
	return retval
}

func (pioa *PIOA) get_potential_transitions() {
	var retval []interface{}

	if pioa.rng_ {
		if pioa.next_state_ == nil {
			pioa.next_state_ = pioa.states_[pioa.current_state_].transition(pioa.rng_)
		}
		retval = append(retval, pioa.next_state_)
	} else {
		for _, transition := range pioa.states_[pioa.current_state_].transitions_.keys() {
			if pioa.states_[pioa.current_state_].transitions_[transition][1] > 0 {
				retval = append(retval, transition)
			}
		}
	}

	return retval
}

func (pioa *PIOA) advance_to_next_state() bool {
	// get the list of possible transitions we could make
	potential_transitions := pioa.get_potential_transitions()
	assert(len(potential_transitions) > 0)

	// attempt to do a normal transition
	var fatal int
	var success bool
	for _, dst_state := range potential_transitions {
		action_block = pioa.determine_action_block(pioa.current_state_, dst_state)

		if success, err := pioa.eval_action_block(action_block); err != nil {
			log.Printf("EXCEPTION: %s", err)
			fatal += 1
		} else if success {
			break
		}
	}

	// if all potential transitions are fatal, attempt the error transition
	if !success && fatal == len(potential_transitions) {
		src_state := pioa.current_state_
		dst_state := pioa.states_[pioa.current_state_].get_error_transition()

		if dst_state {
			action_block = pioa.determine_action_block(src_state, dst_state)
			success = pioa.eval_action_block(action_block)
		}
	}

	if !success {
		return false
	}

	// if we have a successful transition, update our state info.
	pioa.history_len_ += 1
	pioa.current_state_ = dst_state
	pioa.next_state_ = nil

	if pioa.current_state_ == "dead" {
		pioa.success_ = true
	}
	return true
}

func (pioa *PIOA) eval_action_block(action_block) {
	var retval bool

	if len(action_block) == 0 {
		return true
	}

	if len(action_block) >= 1 {
		for _, action_obj := range action_block {
			if action_obj.get_regex_match_incoming() {
				incoming_buffer = pioa.channel_.peek()
				m := re.search(action_obj.get_regex_match_incoming(), incoming_buffer)
				if m {
					retval = pioa.eval_action(action_obj)
				}
			} else {
				retval = pioa.eval_action(action_obj)
			}
			if retval {
				break
			}
		}
	}

	return retval
}

func (pioa *PIOA) transition() {
	var success bool
	if pioa.check_channel_state() {
		pioa.check_rng_state()
		success = pioa.advance_to_next_state()
	}
	return success
}

func (pioa *PIOA) replicate() *PIOA {
	other := NewPIOA(pioa.party_, pioa.first_sender_)
	other.actions_ = pioa.actions_
	other.states_ = pioa.states_
	other.global_ = pioa.global_
	other.local["model_uuid"] = pioa.local["model_uuid"]
	other.port_ = pioa.port_
	other.transport_protocol_ = pioa.transport_protocol_
	return other
}

func (pioa *PIOA) isRunning() {
	return pioa.current_state_ != "dead"
}

func (pioa *PIOA) eval_action(action_obj) {
	module := action_obj.get_module()
	method := action_obj.get_method()
	args := action_obj.get_args()

	i := importlib.import_module("marionette_tg.plugins._" + module)
	method_obj = getattr(i, method)

	return method_obj(pioa.channel_, pioa.marionette_state_, args)
}

func (pioa *PIOA) add_state(name) {
	if !stringSliceContains(pioa.states_.keys(), name) {
		pioa.states_[name] = PAState(name)
	}
}

func (pioa *PIOA) set_multiplexer_outgoing(multiplexer) {
	pioa.global["multiplexer_outgoing"] = multiplexer
}

func (pioa *PIOA) set_multiplexer_incoming(multiplexer) {
	pioa.global["multiplexer_incoming"] = multiplexer
}

func (pioa *PIOA) stop() {
	pioa.current_state_ = "dead"
}

func (pioa *PIOA) set_port(port) {
	pioa.port_ = port
}

func (pioa *PIOA) get_port() int {
	if pioa.port_ != 0 {
		return pioa.port_
	}
	return pioa.local[pioa.port_]
}

type PAState struct {
	name_         string
	transitions_  map[string]interface{}
	format_type_  interface{} // = None
	format_value_ interface{} // = None
	error_state_  interface{} // = None
}

func NewPAState(name string) *PAState {
	return &PAState{
		name_:        name,
		transitions_: make(map[string]interface{}),
	}
}

func (s *PAState) add_transition(dst, action_name, probability) {
	s.transitions_[dst] = []interface{}{action_name, float(probability)}
}

func (s *PAState) set_error_transition(error_state) {
	s.error_state_ = error_state
}

func (s *PAState) get_error_transition() {
	return s.error_state_
}

func (s *PAState) transition(rng) {
	assert(rng != nil || len(s.transitions_) == 1)
	if rng && len(s.transitions_) > 1 {
		coin = rng.random()
		sum = 0
		for _, state := range s.transitions_ {
			if s.transitions_[state][1] == 0 {
				continue
			}
			sum += s.transitions_[state][1]
			if sum >= coin {
				break
			}
		}
	} else {
		state = list(s.transitions_.keys())[0]
	}
	return state
}

// class MarionetteSystemState(object):
//
//     def get_fte_obj(regex, msg_len):
//         fte_key = 'fte_obj-' + regex + str(msg_len)
//         if not self.get_global(fte_key):
//             dfa = regex2dfa.regex2dfa(regex)
//             fte_obj = fte.encoder.DfaEncoder(dfa, msg_len)
//             self.set_global(fte_key, fte_obj)
//
//         return self.get_global(fte_key)
