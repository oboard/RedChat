<template>
  <dialog class="modal" :class="{
    'modal-open': open,
  }">
    <div class="modal-box">
      <h3 class="font-bold text-lg">{{ title }}</h3>
      <slot></slot>
      <div class="modal-action">
        <form method="dialog" class="inline-flex gap-2" @submit="open = false">
          <button class="btn" @click="emit('cancel', $event)">
            取消
          </button>

          <button class="btn" @click="emit('confirm', $event)">
            确定
          </button>
        </form>
      </div>
    </div>
  </dialog>
</template>

<script lang="ts" setup>
const emit = defineEmits(['confirm', 'cancel']);

const open = defineModel<boolean>({
  required: true
});
defineProps({
  title: {
    type: String,
    required: false,
    default: "提示"
  },
});
</script>
