from eiondb import EionClient

client = EionClient(cluster_api_key="eion_cluster_default_key")

print("Registering agents...")

client.register_agent("contract-parser", "Contract Parser", "crud")
client.register_agent("risk-assessor", "Risk Assessor", "crud")
client.register_agent("portfolio-analyzer", "Portfolio Analyzer", "crud")
client.register_agent("market-data-external", "External Market Data Provider", "r", guest=True)

print("Registration complete!")