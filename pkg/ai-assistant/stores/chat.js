let messageIdCounter = 0;

function generateId() {
  return `msg-${ Date.now() }-${ ++messageIdCounter }`;
}

export default {
  namespaced: true,

  state() {
    return {
      drawerOpen: false,
      messages:   [],
      loading:    false,
      error:      null,
    };
  },

  mutations: {
    SET_DRAWER_OPEN(state, open) {
      state.drawerOpen = open;
    },

    ADD_MESSAGE(state, message) {
      state.messages.push(message);
    },

    UPDATE_LAST_ASSISTANT(state, payload) {
      const last = [...state.messages].reverse().find((m) => m.role === 'assistant');

      if (last) {
        if (payload.content !== undefined) {
          last.content = payload.content;
        }
        if (payload.toolCalls !== undefined) {
          last.toolCalls = payload.toolCalls;
        }
      }
    },

    SET_LOADING(state, loading) {
      state.loading = loading;
    },

    SET_ERROR(state, error) {
      state.error = error;
    },

    CLEAR_MESSAGES(state) {
      state.messages = [];
    },
  },

  actions: {
    toggleDrawer({ commit, state }) {
      commit('SET_DRAWER_OPEN', !state.drawerOpen);
    },

    openDrawer({ commit }) {
      commit('SET_DRAWER_OPEN', true);
    },

    closeDrawer({ commit }) {
      commit('SET_DRAWER_OPEN', false);
    },

    clearHistory({ commit }) {
      commit('CLEAR_MESSAGES');
    },

    addUserMessage({ commit }, message) {
      commit('ADD_MESSAGE', {
        id:        generateId(),
        role:      'user',
        content:   message,
        timestamp: Date.now(),
      });
    },

    addAssistantPlaceholder({ commit }) {
      commit('ADD_MESSAGE', {
        id:        generateId(),
        role:      'assistant',
        content:   '',
        toolCalls: [],
        timestamp: Date.now(),
      });
    },

    updateAssistant({ commit }, payload) {
      commit('UPDATE_LAST_ASSISTANT', payload);
    },
  },

  getters: {
    isDrawerOpen:  (state) => state.drawerOpen,
    allMessages:   (state) => state.messages,
    isLoading:     (state) => state.loading,
    currentError:  (state) => state.error,
  },
};
