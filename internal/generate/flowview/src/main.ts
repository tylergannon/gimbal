import { mount } from 'svelte';
import App from './App.svelte';
import '@xyflow/svelte/dist/style.css';
import './style.css';
const page = JSON.parse(document.getElementById('workflow-source')!.textContent!);
mount(App, {target: document.getElementById('app')!, props: {page}});
