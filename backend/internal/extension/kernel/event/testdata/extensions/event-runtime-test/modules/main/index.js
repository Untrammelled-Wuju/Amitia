const { defineEvent } = require("@amitia/plugin-sdk");

module.exports = defineEvent({
  entry: "on-test-event",
  handler: async (input, context) => {
    context.log.info("event received", {
      eventId: input.eventId,
      eventTypeId: input.eventTypeId,
      eventVersion: input.eventVersion,
      deliveryId: input.deliveryId,
      subscriptionId: input.subscriptionId,
      attempt: input.attempt
    });
    return { received: true, eventId: input.eventId };
  }
});
