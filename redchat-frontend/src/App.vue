<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
const serverHost = '127.0.0.1:8080';

// 从 URL 查询参数中获取 conversationId
const urlParams = new URLSearchParams(window.location.search);
const userId = ref(Number.parseFloat(urlParams.get('userId') ?? '0') || 0);

interface Message {
  content: string;
  userId: number;
  conversationId: string;
  time: string;
  uuid: string;
  type: string;
  status: number;
}

// 数据
const messages = ref<Record<string, Message>>({});
const messageContent = ref('');
// const joinConversationId = ref('');
const userConversations = ref([]);
const currentConversationId = ref('');

watch(currentConversationId, () => {
  if (currentConversationId) {
    getChatHistory();
  }
});

// WebSocket 连接对象
let socket: WebSocket | null = null;

// 连接 WebSocket 的函数
const connectWebSocket = () => {
  socket = new WebSocket(`ws://${serverHost}/api/v1/ws?userId=${userId.value}`);
  socket.onopen = () => {
    getChatHistory();
  };
  socket.onmessage = event => {
    const message = JSON.parse(event.data);
    messages.value = { ...messages.value, ...{ [message.uuid]: message } };
  };
  socket.onclose = () => {
    // 连接关闭时尝试重新连接
    console.log('WebSocket 连接关闭，尝试重新连接...');
    setTimeout(connectWebSocket, 3000);
  };
};

const fetch2 = (input: string, init?: RequestInit): Promise<Response> => {
  return fetch(`http://${serverHost}/api/v1` + input, init);
}

// 发送消息方法
const sendMessage = () => {
  const message = {
    content: messageContent.value,
    userId: userId.value,
    conversationId: currentConversationId.value,
    time: new Date().toISOString(),
    uuid: Math.random().toString(36).substring(2, 9),
    type: 'chat',
    status: 1 // 设置初始状态为 sending
  };
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(message));
    messages.value = { ...messages.value, ...{ [message.uuid]: message } };
    messageContent.value = '';
  } else {
    console.error('WebSocket 连接未打开，无法发送消息。');
  }
};

const getChatHistory = () => {
  fetch2(`/history?conversationId=${currentConversationId.value}`)
    .then(response => response.json())
    .then(data => {
      messages.value = data.messages?.map((message: Message) => {
        message.status = 0; // 设置状态为已接收
        return message;
      })
        || {};
    })
    .catch(error => console.error('获取历史聊天记录错误：', error));
};

// const joinConversation = () => {
//   fetch2(`/join?userId=${userId.value}&conversationId=${joinConversationId.value}`)
//     .then(response => response.json())
//     .then(data => {
//       if (data.message === 'joined conversation successfully') {
//         connectWebSocket();
//         getUserConversations();
//       }
//     })
//     .catch(error => console.error('加入会话错误：', error));
// };

const getUserConversations = () => {
  fetch2(`/list?userId=${userId.value}`)
    .then(response => response.json())
    .then(data => {
      userConversations.value = data.conversations || [];
    })
    .catch(error => console.error('获取用户会话错误：', error));
};

// 生命周期钩子
onMounted(() => {
  connectWebSocket();
  getUserConversations();
});
</script>

<template>
  <div class="fixed top-0 left-0 h-[calc(100%-72px)] z-10 w-1/4 bg-base-200 p-4 flex flex-col">
    <h2 class="text-2xl font-semibold mb-4">会话列表</h2>
    <ul class="w-full flex-1 space-y-2">
      <li v-for="conversation in userConversations" :key="conversation" class="w-full">
        <label class="btn pl-2 label cursor-pointer">
          <span class="label-text">{{ conversation }}</span>
          <input type="radio" name="radio-10" class="radio checked:bg-blue-500" v-model="currentConversationId"
            :value="conversation" />
        </label>
        <!-- <input :id="`conversation-radio-${conversation}`" class="radio" type="radio"
                        v-model="currentConversationId" :value="conversation">
                    <label :for="`conversation-radio-${conversation}`"
                        class="w-full flex justify-center cursor-pointer bg-base-300 transition duration-300 ease-in-out">
                        {{ conversation }}
                    </label> -->
      </li>
    </ul>
  </div>
  <div class="absolute top-4 pb-[100px] pl-[calc(25%+32px)]">
    <ul class="space-y-2">
      <li v-for="message of messages" :key="message.uuid"
        :class="message.userId === userId ? 'message sender' : 'message receiver'">
        <span>{{ message.content }}</span>
        <small>{{ message.time }}</small>
        <span v-if="message.status === 1">发送中...</span>
        <span v-if="message.status === 2">发送失败</span>
      </li>
    </ul>
    <form @submit.prevent="sendMessage"
      class="flex items-center gap-2 justify-center fixed bottom-0 left-0 right-0 bg-base-200 bg-opacity-75 p-4 shadow-lg">
      <input type="text" class="input flex-1" v-model="messageContent" placeholder="输入消息">
      <button type="submit" class="px-4 btn btn-primary btn-circle">
        <svg fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" color="white"
            d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
        </svg>
      </button>
    </form>
  </div>
</template>

<style scoped>
.logo {
  height: 6em;
  padding: 1.5em;
  will-change: filter;
  transition: filter 300ms;
}

.logo:hover {
  filter: drop-shadow(0 0 2em #646cffaa);
}

.logo.vue:hover {
  filter: drop-shadow(0 0 2em #42b883aa);
}
</style>
