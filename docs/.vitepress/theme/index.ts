// .vitepress/theme/index.ts
import DefaultTheme from 'vitepress/theme'
import './style.css'

// Custom Components
import TerminalWindow from './components/TerminalWindow.vue'
import FeatureCard from './components/FeatureCard.vue'
import CustomCallout from './components/CustomCallout.vue'
import CodeTabs from './components/CodeTabs.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    // Register global components
    app.component('TerminalWindow', TerminalWindow)
    app.component('FeatureCard', FeatureCard)
    app.component('CustomCallout', CustomCallout)
    app.component('CodeTabs', CodeTabs)
  }
}
