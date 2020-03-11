<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped lang="scss">
h3 {
  margin: 40px 0 0;
}
ul {
  list-style-type: none;
  padding: 0;
}
li {
  display: inline-block;
  margin: 0 10px;
}
a {
  color: #42b983;
}
// .check-img {
//   display: inline-block;
//   background-image: url("../assets/check_mark.svg");
//   width: 24px !important;
//   height: 24px !important;;
// }
</style>

<script lang="ts">
import Vue, { VueConstructor } from "vue";
import Component from "vue-class-component";
import { Prop } from "vue-property-decorator";
import FeatureBlock from "../components/FeatureBlock.vue";

import TextBlock from "../components/TextBlock.vue";

@Component({
  components: { FeatureBlock, TextBlock }
})
export default class FeaturesPage extends Vue {}
</script>

<template>
  <div>
    <TextBlock title="Tenniarb" subtitle="Brain network modeling tool">
      <img width="128" height="128" src="../assets/logo.png" />
    </TextBlock>
    <FeatureBlock title="What it is">
      <template v-slot:descr>Tenniarb is a generic diagram modeling tool capable of doing.</template>
      <template v-slot:items>
        <div>Simple/Complex diagramming</div>
        <div>Category oranization</div>
        <div>Easy/complex styling</div>
        <div>Real time calculations</div>
        <div>Embeded images</div>
        <div>Plain text format</div>
      </template>
    </FeatureBlock>
    <FeatureBlock color="grey" title="Use Cases">
      <template v-slot:descr>Tenniarb is designed for simplicity and could be used for following use-cases</template>
      <template v-slot:items>
        <div>Mind mapping</div>
        <div>Flow diagrams</div>
        <div>Class diagrams</div>
        <div>State chart diagrams</div>
        <div>Presentation authoring</div>
        <div>Task management</div>
        <div>Visual calculations</div>
      </template>
    </FeatureBlock>
    <FeatureBlock title="Use Cases">
      <template v-slot:descr>Tenniarb is a generic diagram modeling tool capable of doing.</template>
      <template v-slot:items>
        <div>Declarative</div>
        <div>Free mind map</div>
        <div>Simple</div>
        <div>Complex</div>
      </template>
    </FeatureBlock>
    <FeatureBlock color="grey" title="Use Cases">
      <template v-slot:descr>Tenniarb is a generic diagram modeling tool capable of doing.</template>
      <template v-slot:items>
        <div>Declarative</div>
        <div>Free mind map</div>
        <div>Simple</div>
        <div>Complex</div>
      </template>
    </FeatureBlock>
    <FeatureBlock title="Use Cases">
      <template v-slot:descr>Tenniarb is a generic diagram modeling tool capable of doing.</template>
      <template v-slot:items>
        <div>Declarative</div>
        <div>Free mind map</div>
        <div>Simple</div>
        <div>Complex</div>
      </template>
    </FeatureBlock>
  </div>
</template>
