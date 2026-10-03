// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAppI18n } from "../i18n";
import AssistantConversationThread, { type AssistantChatTurn } from "./AssistantConversationThread.vue";

afterEach(() => vi.unstubAllGlobals());
function setup() {
  let resize!: ResizeObserverCallback;
  const disconnect = vi.fn();
  vi.stubGlobal("ResizeObserver", class { constructor(callback: ResizeObserverCallback) { resize = callback; } observe() {} disconnect = disconnect; });
  const wrapper = mount(AssistantConversationThread, { props: { name:"助手",welcome:"欢迎",faqs:[],turns:[],busy:false },global:{plugins:[createAppI18n({getItem:()=>null},"zh-CN")]} });
  const element = wrapper.element as HTMLElement;
  let height=800, viewport=200;
  Object.defineProperties(element,{scrollHeight:{get:()=>height},clientHeight:{get:()=>viewport}});
  const turn: AssistantChatTurn={id:"1",question:"问题",answer:"部分回答",state:"generating"};
  return {wrapper,element,turn,disconnect,resize:()=>resize([],{} as ResizeObserver),size:(h:number,v=200)=>{height=h;viewport=v;}};
}
describe("AssistantConversationThread scrolling",()=>{
  it("reveals each new question, streamed answer and final answer after rendering",async()=>{
    const {wrapper,element,turn,size}=setup();
    try {
      await wrapper.setProps({turns:[turn]}); await flushPromises();
      expect(element.scrollTop).toBe(800);
      size(1000); await wrapper.setProps({turns:[{...turn,answer:"更长的回答"}]}); await flushPromises();
      expect(element.scrollTop).toBe(1000);
      size(1200); await wrapper.setProps({turns:[{...turn,answer:"最终回答",state:"completed"}]}); await flushPromises();
      expect(element.scrollTop).toBe(1200);
    }finally{wrapper.unmount();}
  });
  it("preserves reading older messages until the user sends another question",async()=>{
    const {wrapper,element,turn,size}=setup();
    try {
      await wrapper.setProps({turns:[turn]}); await flushPromises();
      element.scrollTop=100;await wrapper.trigger("scroll");
      size(1000);await wrapper.setProps({turns:[{...turn,answer:"新增内容"}]});await flushPromises();
      expect(element.scrollTop).toBe(100);
      await wrapper.setProps({turns:[turn,{...turn,id:"2",question:"下一问"}]});await flushPromises();
      expect(element.scrollTop).toBe(1000);
    }finally{wrapper.unmount();}
  });
  it("keeps the latest answer visible when the viewport or composer changes height and disconnects on unmount",async()=>{
    const {wrapper,element,turn,size,resize,disconnect}=setup();
    await wrapper.setProps({turns:[turn]});await flushPromises();
    size(950,120);resize();await flushPromises();expect(element.scrollTop).toBe(950);
    element.scrollTop=100;await wrapper.trigger("scroll");size(1000,100);resize();await flushPromises();expect(element.scrollTop).toBe(100);
    wrapper.unmount();expect(disconnect).toHaveBeenCalledOnce();
  });
});
