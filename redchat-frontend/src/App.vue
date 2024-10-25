<script setup lang="ts">
import { ref, onMounted, watch, nextTick } from 'vue';
import Modal from './components/modal.vue';
const serverHost = window.location.hostname + ':8080';

import { LRUCache } from 'lru-cache'
const messageCache = new LRUCache({ max: 500, ttl: 1000 * 60 * 5, });

// 从 URL 查询参数中获取 conversationId
const urlParams = new URLSearchParams(window.location.search);
const userId = ref(Number.parseFloat(urlParams.get('userId') ?? '0') || 0);

interface Message {
  content: string;
  userId: number;
  convId: string;
  time: string;
  uuid: string;
  type: string;
  status: number;
}

// 数据
const messages = ref<Record<string, Message>>({});
const messageContent = ref('');
// const joinConversationId = ref('');
const userConversations = ref<string[]>([]);
const currentConversationId = ref('');
const unreads = ref<Record<string, number>>({});

// 页面滚动到底部的函数
const toBottom = () => {
  window.scrollTo(0, document.body.scrollHeight);
}

const reload = async () => {
  await getChatHistory();
  unreads.value[currentConversationId.value] = 0;
  if (window !== undefined) {
    toBottom();
  }
}

watch(currentConversationId, async () => {
  if (currentConversationId) {
    reload();
  }
});

// WebSocket 连接对象
let socket: WebSocket | null = null;

// 连接 WebSocket 的函数
const connectWebSocket = () => {
  socket = new WebSocket(`ws://${serverHost}/api/v1/ws?userId=${userId.value}`);
  socket.onopen = () => {
    reload();
  };
  socket.onmessage = event => {
    const message = JSON.parse(event.data);
    if (message.convId === currentConversationId.value) {
      messages.value = { ...messages.value, ...{ [message.uuid]: message } };
    } else {
      if (!userConversations.value.includes(message.convId)) {
        userConversations.value.push(message.convId);
      }
      // 存入缓存
      messageCache.set(message.convId, { ...messageCache.get(message.convId) || {}, ...{ [message.uuid]: message } });
      unreads.value = { ...unreads.value, ...{ [message.convId]: (unreads.value[message.convId] || 0) + 1 } };
    }
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

const genColor = (uuid: number) => {
  if (uuid == undefined || uuid == null) {
    return "";
  }
  const seed = uuid;
  const colors = [
    "chat-bubble-primary",
    "chat-bubble-secondary",
    "chat-bubble-accent",
    "chat-bubble-neutral",
    "chat-bubble-success",
    "chat-bubble-warning",
    "chat-bubble-error",
  ];
  const color = colors[seed % 7];
  return color;
}


// 发送消息方法
const sendMessage = () => {
  const message = {
    content: messageContent.value,
    userId: userId.value,
    convId: currentConversationId.value,
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
  nextTick(toBottom);
};


const modalTitle = ref<string>("");
const promptModal = ref<boolean>(false);
const confirmModal = ref<boolean>(false);
const inputRef = ref<HTMLInputElement>();
const modalInput = ref<string>("");
const promptConfrim = ref<() => void>();
const promptCancel = ref<() => void>();


// async function customConfirm(message: string): Promise<boolean> {
//   modalTitle.value = message;
//   confirmModal.value = true;
//   return new Promise((resolve) => {
//     promptConfrim.value = () => {
//       resolve(true);
//       confirmModal.value = false;
//     }
//     promptCancel.value = () => {
//       resolve(false);
//       confirmModal.value = false;
//     }
//   });
// }

async function customPrompt(message: string, defaultValue?: string): Promise<string | undefined> {
  modalTitle.value = message;
  modalInput.value = defaultValue ?? "";
  promptModal.value = true;
  // inputRef获取焦点
  inputRef.value?.focus();
  return new Promise((resolve) => {
    promptConfrim.value = () => {
      resolve(modalInput.value);
      promptModal.value = false;
    };
    promptCancel.value = () => {
      resolve(undefined);
      promptModal.value = false;
    };
  });
}

const getChatHistory = async () => {
  if (messageCache.has(currentConversationId.value)) {
    // 如果缓存中有当前会话的历史消息，直接使用缓存数据
    messages.value = messageCache.get(currentConversationId.value) as Record<string, Message>;
  } else {
    // 如果缓存中没有，从服务器获取并缓存起来
    return await fetch2(`/history?convId=${currentConversationId.value}`)
      .then(response => response.json())
      .then(data => {
        const formattedMessages = data.msgs?.map((message: Message) => {
          message.status = 0; // 设置状态为已接收
          return message;
        }) || {};
        messages.value = formattedMessages;
        // 将获取到的消息缓存起来
        messageCache.set(currentConversationId.value, formattedMessages);
      })
      .catch(error => console.error('获取历史聊天记录错误：', error));
  }
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

// 通过时间戳获取时间，如果时间不是很久，就显示多久之前，否则显示具体时间
function getTime(time: number | undefined) {
  if (time === undefined) return "发送中";
  const now = new Date().getTime();
  const diff = now - time;
  if (diff < 1000 * 60) {
    // return `${Math.floor(diff / 1000)}秒前`;
    return "刚刚";
  }
  if (diff < 1000 * 60 * 60) {
    return `${Math.floor(diff / (1000 * 60))}分钟前`;
  }
  if (diff < 1000 * 60 * 60 * 24) {
    return `${Math.floor(diff / (1000 * 60 * 60))}小时前`;
    // } else if (diff < 1000 * 60 * 60 * 24 * 30) {
    //   return `${Math.floor(diff / (1000 * 60 * 60 * 24))}天前`;
  }
  return new Date(time).toLocaleString();
}

const createConversation = (toUserId: number) => {
  fetch2('/create', {
    body: JSON.stringify({
      from: userId.value,
      to: toUserId,
    }),
    method: 'POST',
  }).then(response => response.json())
    .then(data => {
      if (userConversations.value.includes(data?.convId)) {
        return;
      }
      userConversations.value.push(`${data?.convId}`);
    }).catch(error => console.error('创建会话错误：', error));
}

const getUserConversations = async () => {
  return await fetch2(`/list?userId=${userId.value}`)
    .then(response => response.json())
    .then(data => {
      userConversations.value = data.convs || [];
    })
    .catch(error => console.error('获取用户会话错误：', error));
};

const onCreateBtn = async () => {
  let id = await customPrompt("请输入对方的用户 ID", "");
  if (id === undefined) {
    return;
  }
  createConversation(Number.parseInt(id))
}

// 生命周期钩子
onMounted(async () => {

  while (!userId.value) {
    let id = await customPrompt("请输入用户 ID", "")
    if (id !== undefined) {
      userId.value = Number.parseInt(id);
      // 在 url 中添加 userId
      const url = new URL(window.location.href);
      url.searchParams.set('userId', id.toString());
      window.history.replaceState({}, '', url.href);
    }
  }

  await getUserConversations();
  if (userConversations.value.length > 0) {
    currentConversationId.value = userConversations.value[0];
  }
  connectWebSocket();
});
</script>

<template>
  <div class="fixed top-0 left-0 h-[calc(100%-72px)] z-10 w-1/4 bg-base-200 p-4 flex flex-col items-center gap-2">

    <h2 class="text-2xl font-semibold mb-4 hidden md:block">会话列表</h2>
    <button type="button" class="btn btn-primary btn-circle text-xl" @click="onCreateBtn"> + </button>

    <ul class="w-full flex-1 space-y-2">
      <li v-for="conversation in userConversations" :key="conversation" class="w-full">
        <div class="indicator">
          <span :class="{
            'indicator-item badge badge-primary': unreads[conversation] > 0,
            'hidden': (unreads[conversation] || 0) === 0
          }">{{ unreads[conversation] || 0 }}</span>
          <label class=" btn pl-2 label cursor-pointer h-fit">
            <span class="label-text">{{ conversation }}</span>
            <input type="radio" name="radio-10" class="radio checked:bg-blue-500" v-model="currentConversationId"
              :value="conversation" />
          </label>
        </div>
      </li>
    </ul>
  </div>
  <div class="top-4 pb-[100px] pl-[calc(25%)] w-full">
    <div className="chatbox w-full">
      <div v-for="item of messages" :class="{
        'chat': true,
        'chat-end': item.userId === userId,
        'chat-start': item.userId !== userId,
      }" key={item.id}>
        <div className="chat-header">
          <time className="text-xs opacity-50">
            {{ getTime(new Date(item.time).getTime()) }}
          </time>
        </div>
        <div :class='`animate-duration-500 animate-ease-out chat-bubble ${genColor(
          item.userId
        )} animate-fade-in-${item.userId === userId ? "right" : "left"
          }${item.type === "image" ? "  max-w-sm" : ""}`'>
          <!-- <ReactMarkdown
                      // 图片可以点击放大
                      components={{
                        img: ({ node, ...props }) => (
                          <button
                            type="button"
                            className="gap-1 flex flex-row items-center link link-hover"
                            onClick={() => {
                              if (typeof window !== "undefined") {
                                window.open(props.src);
                              }
                            }}
                          >
                            <i className="i-tabler-photo" />
                            查看图片
                          </button>
                          // <img
                          //   className="min-w-8 min-h-8 w-full my-2 rounded hover:shadow-xl cursor-pointer transition-all scale-100 hover:scale-110 hover:rounded-xl"
                          //   src={props.src}
                          //   onClick={() => {
                          //     if (typeof window !== "undefined") {
                          //       window.open(props.src);
                          //     }
                          //   }}
                          // />
                        ),
                        code: ({
                          node,
                          inline,
                          className,
                          children,
                          ...props
                        }) => {
                          const match = /language-(\w+)/.exec(className || "");
                          return !inline && match ? (
                            <CodeBlock language={match[1]}>
                              {String(children).replace(/\n$/, "")}
                            </CodeBlock>
                          ) : (
                            <CodeBlock language={"js"}>
                              {String(children).replace(/\n$/, "")}
                            </CodeBlock>
                          );
                        },
                        a: ({
                          node,
                          // inline,
                          className,
                          children,
                          ...props
                        }) => {
                          return (
                            <div className="flex flex-row gap-1 items-center">
                              {/* 链接图标 */}
                              <svg
                                // 颜色
                                className={
                                  (item.userId === userId
                                    ? "text-primary-content"
                                    : "text-base-content") + " fill-current"
                                }
                                viewBox="0 0 1024 1024"
                                version="1.1"
                                xmlns="http://www.w3.org/2000/svg"
                                width="16"
                                height="16"
                              >
                                <path d="M573.44 640a187.68 187.68 0 0 1-132.8-55.36L416 560l45.28-45.28 24.64 24.64a124.32 124.32 0 0 0 170.08 5.76l1.44-1.28a49.44 49.44 0 0 0 4-3.84l101.28-101.28a124.16 124.16 0 0 0 0-176l-1.92-1.92a124.16 124.16 0 0 0-176 0l-51.68 51.68a49.44 49.44 0 0 0-3.84 4l-20 24.96-49.92-40L480 276.32a108.16 108.16 0 0 1 8.64-9.28l51.68-51.68a188.16 188.16 0 0 1 266.72 0l1.92 1.92a188.16 188.16 0 0 1 0 266.72l-101.28 101.28a112 112 0 0 1-8.48 7.84 190.24 190.24 0 0 1-125.28 48z"></path>
                                <path
                                  d="M350.72 864a187.36 187.36 0 0 1-133.28-55.36l-1.92-1.92a188.16 188.16 0 0 1 0-266.72l101.28-101.28a112 112 0 0 1 8.48-7.84 188.32 188.32 0 0 1 258.08 7.84L608 464l-45.28 45.28-24.64-24.64A124.32 124.32 0 0 0 368 478.88l-1.44 1.28a49.44 49.44 0 0 0-4 3.84l-101.28 101.28a124.16 124.16 0 0 0 0 176l1.92 1.92a124.16 124.16 0 0 0 176 0l51.68-51.68a49.44 49.44 0 0 0 3.84-4l20-24.96 50.08 40-20.8 25.12a108.16 108.16 0 0 1-8.64 9.28l-51.68 51.68A187.36 187.36 0 0 1 350.72 864z"
                                  p-id="4051"
                                ></path>
                              </svg>
                              <a
                                className="link-hover"
                                target="_blank"
                                // 下载
                                download={
                                  item.content.indexOf("api/chat/file") > 0 &&
                                  children
                                }
                                {...props}
                              >
                                {children}
                              </a>
                            </div>
                          );
                        },
                      }}
                    >
                      {item.content}
                    </ReactMarkdown> -->
          {{ item.content }}
        </div>
      </div>
    </div>
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


  <Modal v-model="promptModal" :title="modalTitle" @confirm="promptConfrim" @cancel="promptCancel">
    <input ref="inputRef" type="text" id="folderName" v-model="modalInput" class="input input-bordered w-full mt-4"
      @keydown.enter="promptConfrim" :placeholder="modalTitle" />
  </Modal>
  <Modal v-model="confirmModal" :title="modalTitle" @confirm="promptConfrim" @cancel="promptCancel">
  </Modal>
</template>