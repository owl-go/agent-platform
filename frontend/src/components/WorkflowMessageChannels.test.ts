// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type MessageChannel, type PlatformApi, type ChannelLogin } from "../api/client";
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
const connectedLogin = (provider: string): ChannelLogin => ({ id:"login", provider, status:"connected", qr_content:"", account_id:"bot", account_name:"Bot", suggested_sender_id:"", expires_at:new Date(Date.now()+300000).toISOString() });
const button = (wrapper: ReturnType<typeof widget>, text: string) => wrapper.findAll("button").find(b=>b.text()===text)!;
const field = (wrapper: ReturnType<typeof widget>, label: string) => wrapper.findAll("label").find(l=>l.text().startsWith(label))!.get("input");
vi.mock("qrcode", () => ({ default:{toDataURL:vi.fn(async()=>"data:image/png;base64,cXJjb2Rl")} }));
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
  it("marks a saved provider configured, opens its account and omits reply history", async () => {
    const listDeliveries = vi.fn(async () => []);
    const wrapper = widget({ listMessageChannels: vi.fn(async () => ({ available: true, items: [channel] })), listChannelDeliveries: listDeliveries });
    await flushPromises();
    expect(wrapper.get('[data-provider="telegram"]').text()).toContain("已配置");
    expect(wrapper.get('.channel-card').text()).toContain("已配置");
    expect(wrapper.text()).not.toContain("回复记录");
    expect(wrapper.find('.channel-deliveries').exists()).toBe(false);
    expect(listDeliveries).not.toHaveBeenCalled();
    await wrapper.get('[data-provider="telegram"]').trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("已保存的账号");
    expect(wrapper.findAll('input[type="password"]')).toHaveLength(0);
    wrapper.unmount();
  });
  it("uses direct-only verification guidance for WeChat", async () => {
    const wrapper = widget({ listMessageChannels: vi.fn(async () => ({ available: true, items: [{ ...channel, provider: "wechat", validation_state: "testing", validation_code: "verify example", validation_until: "2026-10-04T08:00:00Z" }] })) });
    await flushPromises();
    expect(wrapper.get('.channel-card').text()).toContain("在允许的私聊中向机器人发送");
    expect(wrapper.get('.channel-card').text()).not.toContain("群聊");
    expect(wrapper.text()).not.toContain("Invalid Date");
    wrapper.unmount();
  });
  it("shows all providers but disables their configuration when the service is unavailable",async()=>{
    const wrapper=widget({listMessageChannels:vi.fn(async()=>({available:false,items:[]}))},"en-US");await flushPromises();expect(wrapper.text()).toContain("The administrator has not enabled");expect(wrapper.findAll(".channel-provider-card")).toHaveLength(13);for(const button of wrapper.findAll(".channel-provider-card"))expect(button.attributes("disabled")).toBeDefined();expect(wrapper.text()).not.toContain("channels.configure");wrapper.unmount();
  });
  it("connects first, hides unsupported direct/group controls and requires Matrix rooms",async()=>{
    const save=vi.fn(async()=>channel);
    const start=vi.fn(async(_workflow,input)=>connectedLogin(input.provider));
    const cancel=vi.fn(async()=>{});
    const wrapper=widget({listMessageChannels:vi.fn(async()=>({available:true,items:[]})),saveMessageChannel:save,startChannelLogin:start,cancelChannelLogin:cancel});await flushPromises();
    expect(wrapper.findAll(".channel-provider-card")).toHaveLength(13);
    await wrapper.get('[data-provider="bluebubbles"]').trigger("click");await flushPromises();
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0);
    await field(wrapper,"HTTPS 服务地址").setValue("https://bridge.test");
    await field(wrapper,"Server Password").setValue("private-draft");
    await button(wrapper,"连接账号").trigger("click");await flushPromises();
    expect(wrapper.text()).toContain("仅支持私聊");
    expect(wrapper.findAll("label").some(l=>l.text().startsWith("允许的群"))).toBe(false);
    await button(wrapper,"取消").trigger("click");
    await wrapper.get('[data-provider="matrix"]').trigger("click");await flushPromises();
    expect(wrapper.html()).not.toContain("private-draft");
    await field(wrapper,"HTTPS 服务地址").setValue("https://matrix.test");
    await field(wrapper,"Access Token").setValue("matrix-token");
    await button(wrapper,"连接账号").trigger("click");await flushPromises();
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0);
    await field(wrapper,"允许的发送者").setValue("@alice:example.test");
    expect(button(wrapper,"保存").attributes("disabled")).toBeDefined();
    await field(wrapper,"允许的房间").setValue("!room:example.test");
    await button(wrapper,"保存").trigger("click");await flushPromises();
    expect(save).toHaveBeenCalledWith("workflow",expect.objectContaining({provider:"matrix",credentials:{},login_id:"login",audience:{sender_ids:["@alice:example.test"],group_ids:["!room:example.test"],allow_direct:false}}),expect.any(AbortSignal));wrapper.unmount();
  });

  it.each([
    ["telegram", "Telegram", ["Bot Token"]], ["discord", "Discord", ["Bot Token"]], ["slack", "Slack", ["Bot Token", "Signing Secret"]],
    ["dingtalk", "DingTalk", ["Client ID", "Client Secret", "Enterprise Corp ID"]], ["feishu", "Feishu", ["App ID", "App Secret", "Tenant Key"]],
    ["matrix", "Matrix", ["HTTPS endpoint", "Access Token"]], ["whatsapp", "WhatsApp", ["Access Token", "App Secret", "Verify Token", "Phone Number ID", "Business Account ID"]],
    ["signal", "Signal", ["HTTPS endpoint", "Bridge Token", "Account ID"]], ["wecom", "WeCom", ["Bot ID", "Bot Secret"]],
    ["qqbot", "QQ Bot", ["App ID", "App Secret"]],
    ["bluebubbles", "BlueBubbles (iMessage)", ["HTTPS endpoint", "Server Password"]], ["yuanbao", "Yuanbao", ["App Key", "App Secret"]],
  ])("renders the actual %s form and clears secret drafts on close", async (provider, name, labels) => {
    const wrapper = mount(WorkflowMessageChannels, {
      props: { workflowId: "workflow" },
      global: { plugins: [createAppI18n({ getItem: () => "en-US" }, "en-US")], provide: { [platformApiKey as symbol]: { listMessageChannels: vi.fn(async () => ({ available: true, items: [] })) } } },
    });
    try {
      await flushPromises();
      await wrapper.get(`[data-provider="${provider}"]`).trigger("click"); await flushPromises();
      if (provider === "qqbot") {
        const manual = [...document.body.querySelectorAll("button")].find(b=>b.textContent?.trim()==="Application credentials")!;
        manual.click();await flushPromises();
      }
      const dialog = document.body.querySelector('[role="dialog"]')!;
      expect(dialog).not.toBeNull();
      expect(dialog.textContent).toContain(`Configure ${name}`);
      for (const label of labels) expect(dialog.textContent).toContain(label);
      const secret = dialog.querySelector<HTMLInputElement>('input[type="password"]')!;
      expect(secret).not.toBeNull();
      secret.value = "secret-draft";secret.dispatchEvent(new Event("input", { bubbles: true }));await flushPromises();
      wrapper.findComponent({ name: "ElDialog" }).vm.$emit("update:modelValue", false);await flushPromises();
      await wrapper.get(`[data-provider="${provider}"]`).trigger("click");await flushPromises();
      if (provider === "qqbot") { [...document.body.querySelectorAll("button")].find(b=>b.textContent?.trim()==="Application credentials")!.click();await flushPromises(); }
      expect(document.body.querySelector<HTMLInputElement>('[role="dialog"] input[type="password"]')!.value).toBe("");
    } finally { wrapper.unmount(); }
  });

  it.each(["wechat", "qqbot"])("uses real QR progress for %s, then saves a server-side authorization", async(provider)=>{
    vi.useFakeTimers();
    const waiting: ChannelLogin = {...connectedLogin(provider),status:"waiting",qr_content:"https://provider.test/qr"};
    const start=vi.fn(async()=>waiting),poll=vi.fn(async()=>({...connectedLogin(provider),suggested_sender_id:"scanner"}));
    const save=vi.fn(async()=>channel),cancel=vi.fn(async()=>{});
    const wrapper=widget({listMessageChannels:vi.fn(async()=>({available:true,items:[]})),startChannelLogin:start,pollChannelLogin:poll,saveMessageChannel:save,cancelChannelLogin:cancel});
    try {
      await flushPromises();await wrapper.get(`[data-provider="${provider}"]`).trigger("click");await flushPromises();
      expect(wrapper.findAll('input[type="password"]')).toHaveLength(0);
      expect(wrapper.findAll("label").some(l=>l.text().startsWith("允许的发送者"))).toBe(false);
      await button(wrapper,"生成二维码").trigger("click");await flushPromises();
      expect(start).toHaveBeenCalledWith("workflow",expect.objectContaining({provider,method:"qr",credentials:{}}),expect.any(AbortSignal));
      expect(wrapper.find(".channel-qr img").exists()).toBe(true);
      await vi.advanceTimersByTimeAsync(2000);await flushPromises();
      expect(wrapper.text()).toContain("账号授权成功");expect(wrapper.find(".channel-qr img").exists()).toBe(false);
      expect((field(wrapper,"允许的发送者").element as HTMLInputElement).value).toBe("scanner");
      await button(wrapper,"保存").trigger("click");await flushPromises();
      expect(save).toHaveBeenCalledWith("workflow",expect.objectContaining({provider,login_id:"login",credentials:{}}),expect.any(AbortSignal));
    } finally {wrapper.unmount();vi.useRealTimers();}
  });
  it("expires QR codes and cancels an unfinished authorization when closed", async()=>{
    vi.useFakeTimers();
    const waiting: ChannelLogin={...connectedLogin("wechat"),status:"waiting",qr_content:"https://provider.test/qr",expires_at:new Date(Date.now()+1000).toISOString()};
    const cancel=vi.fn(async()=>{}),poll=vi.fn(async()=>waiting);
    const wrapper=widget({listMessageChannels:vi.fn(async()=>({available:true,items:[]})),startChannelLogin:vi.fn(async()=>waiting),cancelChannelLogin:cancel,pollChannelLogin:poll});
    try {
      await flushPromises();await wrapper.get('[data-provider="wechat"]').trigger("click");await button(wrapper,"生成二维码").trigger("click");await flushPromises();
      await vi.advanceTimersByTimeAsync(1000);await flushPromises();expect(wrapper.text()).toContain("已过期");expect(wrapper.find(".channel-qr img").exists()).toBe(false);expect(poll).not.toHaveBeenCalled();
      await button(wrapper,"取消").trigger("click");expect(cancel).toHaveBeenCalledWith("workflow","login");
    } finally {wrapper.unmount();vi.useRealTimers();}
  });

});
