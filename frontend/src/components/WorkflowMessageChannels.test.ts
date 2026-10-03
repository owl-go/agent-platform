// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type MessageChannel, type PlatformApi } from "../api/client";
import { createAppI18n } from "../i18n";
import WorkflowMessageChannels from "./WorkflowMessageChannels.vue";

const channel: MessageChannel = { workflow_id:"workflow",account_id:"123",tenant_id:"",region:"",validation_code:"",error_code:"",callback_url:"", id:"channel", name:"Test", provider:"telegram", version:3, config_version:1, enabled:false, validation_state:"passed", health:"disconnected", account_name:"test_bot", audience:{sender_ids:["123"], group_ids:[], allow_direct:true} };
function widget(api: Partial<PlatformApi>, locale: "zh-CN" | "en-US" = "zh-CN") {
  return mount(WorkflowMessageChannels,{props:{workflowId:"workflow"},global:{plugins:[createAppI18n({getItem:()=>locale},locale)],provide:{[platformApiKey as symbol]:api},stubs:{
    ElButton:{template:'<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',props:["disabled"]},
    ElAlert:{template:'<p role="alert">{{title}}</p>',props:["title"]}, ElEmpty:{template:'<p>{{description}}</p>',props:["description"]}, ElTag:{template:'<span><slot /></span>'},
    ElForm:{template:'<form><slot /></form>'}, ElFormItem:{template:'<label>{{label}}<slot /></label>',props:["label"]},
    ElInput:{template:'<input :value="modelValue" :type="type" :disabled="disabled" @input="$emit(\'update:modelValue\', $event.target.value)" />',props:["modelValue","type","disabled"]},
    ElSelect:{template:'<select :value="modelValue" :disabled="disabled" @change="$emit(\'update:modelValue\', $event.target.value)"><slot /></select>',props:["modelValue","disabled"]},
    ElOption:{template:'<option :value="value">{{label}}</option>',props:["value","label"]},
    ElCheckbox:{template:'<label><input type="checkbox" :checked="modelValue" :disabled="disabled" @change="$emit(\'update:modelValue\', $event.target.checked)"/><slot/></label>',props:["modelValue","disabled"]},
    ElDialog:{template:'<div v-if="modelValue"><slot /><slot name="footer" /></div>',props:["modelValue"]},
    ConfirmDialog:{template:'<div v-if="open" class="confirmation"><p>{{message}}</p><button class="confirm" @click="$emit(\'confirm\')">{{confirmLabel}}</button></div>',props:["open","message","confirmLabel"]},
  }}});
}
describe("Workflow message channels",()=>{
  it("shows all thirteen provider configuration entries before any account is saved", async () => {
    const wrapper = mount(WorkflowMessageChannels, {
      props: { workflowId: "workflow" },
      global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: { listMessageChannels: vi.fn(async () => ({ available: true, items: [] })) } } },
    });
    try {
      await flushPromises();
      expect(wrapper.findAll(".channel-provider-card")).toHaveLength(13);
      expect(wrapper.text()).not.toContain("common.refresh");
      await wrapper.get('[data-provider="telegram"]').trigger("click");
      await flushPromises();
      expect(document.body.querySelector('[role="dialog"] input[type="password"]')).not.toBeNull();
      expect(document.body.textContent).toContain("Bot Token");
    } finally { wrapper.unmount(); }
  });
  it("keeps configuration disabled until verification and confirms shared Workspace access",async()=>{
    const control = vi.fn(async()=>({...channel,enabled:true,version:4}));
    const wrapper=widget({listMessageChannels:vi.fn(async()=>({available:true,items:[channel]})),controlMessageChannel:control}); await flushPromises();
    const enable=wrapper.findAll("button").find(b=>b.text()==="启用")!;await enable.trigger("click");expect(control).not.toHaveBeenCalled();expect(wrapper.find(".confirmation").text()).toContain("共享文件");await wrapper.get(".confirm").trigger("click");await flushPromises();expect(control).toHaveBeenCalledWith("workflow","channel",3,"enable",expect.any(AbortSignal));wrapper.unmount();
  });
  it("warns about duplicate sends for an unconfirmed delivery and never reruns",async()=>{
    const retry=vi.fn(async()=>{}); const active={...channel,enabled:true}; const wrapper=widget({listMessageChannels:vi.fn(async()=>({available:true,items:[active]})),listChannelDeliveries:vi.fn(async()=>[{id:"delivery",state:"outcome_unknown",chunk:1,created_at:"2026-10-03T00:00:00Z"} as never]),retryChannelDelivery:retry});await flushPromises();await wrapper.findAll("button").find(b=>b.text()==="回复记录")!.trigger("click");await flushPromises();await wrapper.findAll("button").find(b=>b.text()==="重新发送")!.trigger("click");expect(wrapper.find(".confirmation").text()).toContain("可能产生重复回复");expect(retry).not.toHaveBeenCalled();await wrapper.get(".confirm").trigger("click");await flushPromises();expect(retry).toHaveBeenCalledWith("workflow","channel","delivery",3,true,expect.any(AbortSignal));wrapper.unmount();
  });
  it("shows all providers but disables their configuration when the service is unavailable",async()=>{
    const wrapper=widget({listMessageChannels:vi.fn(async()=>({available:false,items:[]}))},"en-US");await flushPromises();expect(wrapper.text()).toContain("The administrator has not enabled");expect(wrapper.findAll(".channel-provider-card")).toHaveLength(13);for(const button of wrapper.findAll(".channel-provider-card"))expect(button.attributes("disabled")).toBeDefined();expect(wrapper.text()).not.toContain("channels.configure");wrapper.unmount();
  });
  it("offers all thirteen channels and enforces provider audience constraints",async()=>{
    const save=vi.fn(async()=>channel);
    const wrapper=widget({listMessageChannels:vi.fn(async()=>({available:true,items:[]})),saveMessageChannel:save});await flushPromises();
    expect(wrapper.findAll(".channel-provider-card")).toHaveLength(13);
    expect(wrapper.text()).toContain("BlueBubbles");expect(wrapper.text()).toContain("元宝");
    await wrapper.get('[data-provider="bluebubbles"]').trigger("click");await flushPromises();
    const labels=wrapper.findAll("label");const password=labels.find(l=>l.text()==="Server Password")!.get("input");expect(password.attributes("type")).toBe("password");
    expect(wrapper.get('input[type="checkbox"]').attributes("disabled")).toBeDefined();expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true);
    await password.setValue("private-draft");
    await wrapper.findAll("button").find(b=>b.text()==="取消")!.trigger("click");
    await wrapper.get('[data-provider="matrix"]').trigger("click");await flushPromises();expect((wrapper.get('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false);
    expect(wrapper.html()).not.toContain("private-draft");
    await wrapper.findAll("label").find(l=>l.text().startsWith("名称"))!.get("input").setValue("Matrix bot");
    await wrapper.findAll("label").find(l=>l.text().startsWith("允许的发送者"))!.get("input").setValue("@alice:example.test");
    await wrapper.findAll("label").find(l=>l.text().startsWith("允许的群"))!.get("input").setValue("");
    expect(wrapper.findAll("button").find(b=>b.text()==="保存")!.attributes("disabled")).toBeDefined();
    await wrapper.findAll("label").find(l=>l.text().startsWith("允许的群"))!.get("input").setValue("!room:example.test");
    await wrapper.findAll("button").find(b=>b.text()==="保存")!.trigger("click");await flushPromises();
    expect(save).toHaveBeenCalledWith("workflow",expect.objectContaining({provider:"matrix",audience:{sender_ids:["@alice:example.test"],group_ids:["!room:example.test"],allow_direct:false}}),expect.any(AbortSignal));wrapper.unmount();
  });

  it.each([
    ["telegram", "Telegram", ["Bot Token"]], ["discord", "Discord", ["Bot Token"]], ["slack", "Slack", ["Bot Token", "Signing Secret"]],
    ["dingtalk", "DingTalk", ["Client ID", "Client Secret", "Enterprise Corp ID"]], ["feishu", "Feishu", ["App ID", "App Secret", "Tenant Key"]],
    ["matrix", "Matrix", ["HTTPS endpoint", "Access Token"]], ["whatsapp", "WhatsApp", ["Access Token", "App Secret", "Verify Token", "Phone Number ID", "Business Account ID"]],
    ["signal", "Signal", ["HTTPS endpoint", "Bridge Token", "Account ID"]], ["wecom", "WeCom", ["Bot ID", "Bot Secret"]],
    ["wechat", "WeChat", ["Bot Token", "Account ID", "User ID"]], ["qqbot", "QQ Bot", ["App ID", "App Secret"]],
    ["bluebubbles", "BlueBubbles (iMessage)", ["HTTPS endpoint", "Server Password"]], ["yuanbao", "Yuanbao", ["App Key", "App Secret"]],
  ])("renders the actual %s form and clears secret drafts on close", async (provider, name, labels) => {
    const wrapper = mount(WorkflowMessageChannels, {
      props: { workflowId: "workflow" },
      global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: { listMessageChannels: vi.fn(async () => ({ available: true, items: [] })) } } },
    });
    try {
      await flushPromises();
      await wrapper.get(`[data-provider="${provider}"]`).trigger("click"); await flushPromises();
      const dialog = document.body.querySelector('[role="dialog"]')!;
      expect(dialog).not.toBeNull();
      expect(dialog.textContent).toContain(`Configure ${name}`);
      for (const label of labels) expect(dialog.textContent).toContain(label);
      const secret = dialog.querySelector<HTMLInputElement>('input[type="password"]')!;
      expect(secret).not.toBeNull();
      secret.value = "secret-draft";secret.dispatchEvent(new Event("input", { bubbles: true }));await flushPromises();
      wrapper.findComponent({ name: "ElDialog" }).vm.$emit("update:modelValue", false);await flushPromises();
      await wrapper.get(`[data-provider="${provider}"]`).trigger("click");await flushPromises();
      expect(document.body.querySelector<HTMLInputElement>('[role="dialog"] input[type="password"]')!.value).toBe("");
    } finally { wrapper.unmount(); }
  });

});
